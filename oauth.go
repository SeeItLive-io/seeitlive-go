package seeitlive

import (
	"context"
	"strings"
)

// OAuth groups the OAuth-related API calls.
type OAuth struct {
	http *httpClient
}

// rawTokenInfo mirrors the snake_case wire shape of GET /oauth/verify.
type rawTokenInfo struct {
	Valid     bool     `json:"valid"`
	ClientID  string   `json:"client_id"`
	TeamID string   `json:"team_id"`
	Scopes    []string `json:"scopes"`
	Plan      Plan     `json:"plan"`
	ExpiresAt *string  `json:"expires_at"`
}

// Verify checks the client's Bearer token. GET /oauth/verify. Only
// meaningful when the Client was constructed with an OAuthToken.
func (o *OAuth) Verify(ctx context.Context) (*TokenInfo, error) {
	var raw rawTokenInfo
	if err := o.http.request(ctx, "/oauth/verify", requestOptions{}, &raw); err != nil {
		return nil, err
	}
	return &TokenInfo{
		Valid:     raw.Valid,
		ClientID:  raw.ClientID,
		TeamID: raw.TeamID,
		Scopes:    raw.Scopes,
		Plan:      raw.Plan,
		ExpiresAt: raw.ExpiresAt,
	}, nil
}

// rawTokenResponse mirrors the snake_case wire shape of POST /oauth/token.
type rawTokenResponse struct {
	AccessToken string   `json:"access_token"`
	TokenType   string   `json:"token_type"`
	ClientID    string   `json:"client_id"`
	TeamID   string   `json:"team_id"`
	Scopes      []string `json:"scopes"`
	Plan        Plan     `json:"plan"`
}

// ExchangeCode exchanges an OAuth authorization code for an access token.
// POST /oauth/token. This is a standalone function (not a Client method):
// it is called before any token exists and sends no bearer auth — the
// client credentials travel in the POST body instead.
func ExchangeCode(ctx context.Context, params ExchangeCodeParams) (*TokenResponse, error) {
	baseURL := params.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	h := newHTTPClient(baseURL, "", "")

	body := map[string]interface{}{
		"grant_type":    "authorization_code",
		"code":          params.Code,
		"client_id":     params.ClientID,
		"client_secret": params.ClientSecret,
		"redirect_uri":  params.RedirectURI,
	}
	if params.CodeVerifier != "" {
		body["code_verifier"] = params.CodeVerifier
	}

	var raw rawTokenResponse
	if err := h.request(ctx, "/oauth/token", requestOptions{method: "POST", body: body}, &raw); err != nil {
		return nil, err
	}
	return &TokenResponse{
		AccessToken: raw.AccessToken,
		TokenType:   raw.TokenType,
		ClientID:    raw.ClientID,
		TeamID:   raw.TeamID,
		Scopes:      raw.Scopes,
		Plan:        raw.Plan,
	}, nil
}
