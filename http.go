package seeitlive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// httpClient is the shared low-level transport used by both the
// authenticated Client resources and the standalone ExchangeCode function.
type httpClient struct {
	baseURL       string
	authHeader    string
	signingSecret string
	client        *http.Client
}

func newHTTPClient(baseURL, authHeader, signingSecret string) *httpClient {
	return &httpClient{baseURL: baseURL, authHeader: authHeader, signingSecret: signingSecret, client: http.DefaultClient}
}

type requestOptions struct {
	method string // defaults to GET when empty
	body   interface{}
}

// request performs an HTTP call to baseURL+path, decoding a JSON response
// body into out (when out is non-nil and the response isn't 204/empty).
// Non-2xx responses are converted into the typed error taxonomy.
func (h *httpClient) request(ctx context.Context, path string, opts requestOptions, out interface{}) error {
	method := opts.method
	if method == "" {
		method = http.MethodGet
	}

	var encoded []byte
	hasBody := opts.body != nil
	if hasBody {
		var err error
		encoded, err = json.Marshal(opts.body)
		if err != nil {
			return err
		}
	}

	var bodyReader io.Reader
	if hasBody {
		bodyReader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, h.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	if h.authHeader != "" {
		req.Header.Set("Authorization", h.authHeader)
	}
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	if h.signingSecret != "" {
		sigHeader, tsHeader := signRequest(h.signingSecret, method, path, encoded, time.Now().Unix())
		req.Header.Set("X-SeeItLive-Signature", sigHeader)
		req.Header.Set("X-SeeItLive-Timestamp", tsHeader)
	}

	res, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return handleErrorResponse(res)
	}
	if res.StatusCode == http.StatusNoContent || out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

type errorBody struct {
	Message string `json:"message"`
	Code    string `json:"code"`
	Field   string `json:"field"`
}

func handleErrorResponse(res *http.Response) error {
	data, _ := io.ReadAll(res.Body)
	var body errorBody
	_ = json.Unmarshal(data, &body) // non-JSON error bodies are tolerated

	message := body.Message
	if message == "" {
		message = fmt.Sprintf("HTTP %d", res.StatusCode)
	}

	base := APIError{StatusCode: res.StatusCode, Code: body.Code, Message: message}

	switch res.StatusCode {
	case http.StatusBadRequest:
		return &ValidationError{APIError: base, Field: body.Field}
	case http.StatusUnauthorized:
		return &AuthenticationError{APIError: base}
	case http.StatusForbidden:
		return &PermissionError{APIError: base}
	case http.StatusNotFound:
		return &NotFoundError{APIError: base}
	case http.StatusTooManyRequests:
		var retryAfter *int
		if h := res.Header.Get("Retry-After"); h != "" {
			if v, err := strconv.Atoi(h); err == nil {
				retryAfter = &v // preserve a legitimate 0 (retry now); absent/unparsable -> nil
			}
		}
		return &RateLimitError{APIError: base, RetryAfter: retryAfter}
	default:
		return &base
	}
}
