package seeitlive_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	seeitlive "github.com/seeitlive/seeitlive-go"
)

var testAPIKey = "sil_live_" + strings.Repeat("a", 32)
var testOAuthToken = "sil_live_oauth_" + strings.Repeat("a", 40)

func testPlan() seeitlive.Plan {
	return seeitlive.Plan{PlanKey: "free", PlanName: "Free"}
}

// (a) env inference + XOR-credential error.
func TestNewEnvInferenceAndCredentialXOR(t *testing.T) {
	testEnv, err := seeitlive.New(seeitlive.Config{APIKey: "sil_test_" + strings.Repeat("a", 32)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if testEnv.Env != seeitlive.EnvTest {
		t.Errorf("expected env %q, got %q", seeitlive.EnvTest, testEnv.Env)
	}

	liveEnv, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if liveEnv.Env != seeitlive.EnvLive {
		t.Errorf("expected env %q, got %q", seeitlive.EnvLive, liveEnv.Env)
	}

	if _, err := seeitlive.New(seeitlive.Config{}); err == nil {
		t.Error("expected error when neither APIKey nor OAuthToken provided")
	}

	if _, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, OAuthToken: "x"}); err == nil {
		t.Error("expected error when both APIKey and OAuthToken provided")
	}

	// explicit Env override wins over inference
	override, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, Env: seeitlive.EnvTest})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if override.Env != seeitlive.EnvTest {
		t.Errorf("expected explicit env override %q, got %q", seeitlive.EnvTest, override.Env)
	}
}

// (b) Sessions.Create request shape (method, path, bearer header, JSON body) + response parse.
func TestSessionsCreate(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotContentType string
	var gotBody []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":             "s1",
			"status":         "pending",
			"inviteCode":     "abc",
			"inviteUrl":      "u",
			"inviteShortUrl": "https://sitl.cc/s/aB3xK9",
			"agentViewUrl":   "a",
			"expiresAt":      "2026-01-01T00:00:00Z",
		})
	}))
	defer ts.Close()

	client, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := client.Sessions.Create(context.Background(), seeitlive.CreateSessionParams{Channel: seeitlive.ChannelLink})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v1/sessions" {
		t.Errorf("path = %q, want /v1/sessions", gotPath)
	}
	if gotAuth != "Bearer "+testAPIKey {
		t.Errorf("Authorization header = %q, want Bearer %s", gotAuth, testAPIKey)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type header = %q, want application/json", gotContentType)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	// NO teamId field, and only channel is set (no other optional fields sent).
	if len(body) != 1 {
		t.Errorf("body = %v, want exactly {channel: link}", body)
	}
	if body["channel"] != "link" {
		t.Errorf("body[channel] = %v, want link", body["channel"])
	}
	if _, ok := body["teamId"]; ok {
		t.Error("body must not contain teamId")
	}

	if sess.ID != "s1" || sess.Status != "pending" || sess.InviteCode != "abc" ||
		sess.InviteURL != "u" || sess.InviteShortURL == nil || *sess.InviteShortURL != "https://sitl.cc/s/aB3xK9" ||
		sess.AgentViewURL != "a" || sess.ExpiresAt == nil || *sess.ExpiresAt != "2026-01-01T00:00:00Z" {
		t.Errorf("unexpected session: %+v", sess)
	}
}

// (c) Sessions.Status path + parse.
func TestSessionsStatus(t *testing.T) {
	var gotMethod, gotPath string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":            "s1",
			"status":        "live",
			"startedAt":     "t",
			"endedAt":       nil,
			"producerCount": 1,
			"hasVideo":      true,
		})
	}))
	defer ts.Close()

	client, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status, err := client.Sessions.Status(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/v1/sessions/s1/status" {
		t.Errorf("path = %q, want /v1/sessions/s1/status", gotPath)
	}
	if status.ProducerCount != 1 {
		t.Errorf("producerCount = %d, want 1", status.ProducerCount)
	}
	if !status.HasVideo {
		t.Error("hasVideo = false, want true")
	}
	if status.EndedAt != nil {
		t.Errorf("endedAt = %v, want nil", status.EndedAt)
	}
}

// (d) OAuth.Verify snake_case -> idiomatic mapping.
func TestOAuthVerify(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/oauth/verify" {
			t.Errorf("path = %q, want /v1/oauth/verify", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":      true,
			"client_id":  "c",
			"team_id": "p",
			"scopes":     []string{"sessions:write"},
			"plan":       testPlan(),
			"expires_at": "2026-01-01",
		})
	}))
	defer ts.Close()

	client, err := seeitlive.New(seeitlive.Config{OAuthToken: testOAuthToken, BaseURL: ts.URL + "/v1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := client.OAuth.Verify(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !info.Valid {
		t.Error("valid = false, want true")
	}
	if info.ClientID != "c" {
		t.Errorf("ClientID = %q, want c", info.ClientID)
	}
	if info.TeamID != "p" {
		t.Errorf("TeamID = %q, want p", info.TeamID)
	}
	if len(info.Scopes) != 1 || info.Scopes[0] != "sessions:write" {
		t.Errorf("Scopes = %v", info.Scopes)
	}
	if info.Plan.PlanKey != "free" {
		t.Errorf("Plan.PlanKey = %q, want free", info.Plan.PlanKey)
	}
}

// (e) status -> error mapping, including 429 with Retry-After: 12 and Retry-After: 0.
func TestErrorMapping(t *testing.T) {
	newErrorServer := func(status int, body map[string]interface{}, headers map[string]string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for k, v := range headers {
				w.Header().Set(k, v)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(body)
		}))
	}

	cases := []struct {
		name   string
		status int
		body   map[string]interface{}
	}{
		{"401 -> AuthenticationError", http.StatusUnauthorized, map[string]interface{}{"message": "x"}},
		{"403 -> PermissionError", http.StatusForbidden, map[string]interface{}{"message": "x"}},
		{"404 -> NotFoundError", http.StatusNotFound, map[string]interface{}{"message": "x"}},
		{"400 -> ValidationError", http.StatusBadRequest, map[string]interface{}{"message": "x", "field": "channel"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := newErrorServer(tc.status, tc.body, nil)
			defer ts.Close()

			client, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			_, err = client.Sessions.Status(context.Background(), "s")
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			switch tc.status {
			case http.StatusUnauthorized:
				var target *seeitlive.AuthenticationError
				if !errors.As(err, &target) {
					t.Errorf("expected *AuthenticationError, got %T", err)
				}
			case http.StatusForbidden:
				var target *seeitlive.PermissionError
				if !errors.As(err, &target) {
					t.Errorf("expected *PermissionError, got %T", err)
				}
			case http.StatusNotFound:
				var target *seeitlive.NotFoundError
				if !errors.As(err, &target) {
					t.Errorf("expected *NotFoundError, got %T", err)
				}
			case http.StatusBadRequest:
				var target *seeitlive.ValidationError
				if !errors.As(err, &target) {
					t.Errorf("expected *ValidationError, got %T", err)
				} else if target.Field != "channel" {
					t.Errorf("Field = %q, want channel", target.Field)
				}
			}

			var apiErr *seeitlive.APIError
			if !errors.As(err, &apiErr) {
				t.Errorf("expected error to be an *APIError, got %T", err)
			} else if apiErr.StatusCode != tc.status {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tc.status)
			}
		})
	}

	t.Run("429 with Retry-After: 12 -> RateLimitError.RetryAfter = 12", func(t *testing.T) {
		ts := newErrorServer(http.StatusTooManyRequests, map[string]interface{}{"message": "slow"}, map[string]string{"Retry-After": "12"})
		defer ts.Close()

		client, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = client.Sessions.Status(context.Background(), "s")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var rle *seeitlive.RateLimitError
		if !errors.As(err, &rle) {
			t.Fatalf("expected *RateLimitError, got %T", err)
		}
		if rle.RetryAfter == nil || *rle.RetryAfter != 12 {
			t.Errorf("RetryAfter = %v, want 12", rle.RetryAfter)
		}
	})

	// Critical edge case: Retry-After: 0 is a LEGITIMATE value (retry immediately) and
	// must be preserved, not coerced to "absent" the way a naive zero-value int would.
	t.Run("429 with Retry-After: 0 -> RetryAfter preserved as 0, not absent", func(t *testing.T) {
		ts := newErrorServer(http.StatusTooManyRequests, map[string]interface{}{"message": "slow"}, map[string]string{"Retry-After": "0"})
		defer ts.Close()

		client, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = client.Sessions.Status(context.Background(), "s")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var rle *seeitlive.RateLimitError
		if !errors.As(err, &rle) {
			t.Fatalf("expected *RateLimitError, got %T", err)
		}
		if rle.RetryAfter == nil {
			t.Fatal("RetryAfter = nil, want pointer to 0 (legitimate retry-now value)")
		}
		if *rle.RetryAfter != 0 {
			t.Errorf("RetryAfter = %d, want 0", *rle.RetryAfter)
		}
	})

	// Absent Retry-After header must map to nil (undefined), not to 0.
	t.Run("429 without Retry-After header -> RetryAfter is nil", func(t *testing.T) {
		ts := newErrorServer(http.StatusTooManyRequests, map[string]interface{}{"message": "slow"}, nil)
		defer ts.Close()

		client, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = client.Sessions.Status(context.Background(), "s")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var rle *seeitlive.RateLimitError
		if !errors.As(err, &rle) {
			t.Fatalf("expected *RateLimitError, got %T", err)
		}
		if rle.RetryAfter != nil {
			t.Errorf("RetryAfter = %v, want nil", rle.RetryAfter)
		}
	})

	t.Run("other non-2xx -> base APIError", func(t *testing.T) {
		ts := newErrorServer(http.StatusInternalServerError, map[string]interface{}{"message": "boom"}, nil)
		defer ts.Close()

		client, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = client.Sessions.Status(context.Background(), "s")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var apiErr *seeitlive.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected *APIError, got %T", err)
		}
		if apiErr.StatusCode != 500 || apiErr.Message != "boom" {
			t.Errorf("unexpected APIError: %+v", apiErr)
		}
	})
}

// (f) ExchangeCode posts to /oauth/token, NO auth header, grant_type=authorization_code, token mapped.
func TestExchangeCode(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "sil_live_oauth_x",
			"token_type":   "Bearer",
			"client_id":    "c",
			"team_id":   "p",
			"scopes":       []string{},
			"plan":         testPlan(),
		})
	}))
	defer ts.Close()

	tok, err := seeitlive.ExchangeCode(context.Background(), seeitlive.ExchangeCodeParams{
		Code:         "code123",
		ClientID:     "c",
		ClientSecret: "s",
		RedirectURI:  "https://x/cb",
		BaseURL:      ts.URL + "/v1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v1/oauth/token" {
		t.Errorf("path = %q, want /v1/oauth/token", gotPath)
	}
	if gotAuth != "" {
		t.Errorf("Authorization header = %q, want empty (no bearer)", gotAuth)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if body["grant_type"] != "authorization_code" {
		t.Errorf("grant_type = %v, want authorization_code", body["grant_type"])
	}
	if _, ok := body["code_verifier"]; ok {
		t.Error("code_verifier must be omitted when not provided")
	}

	if tok.AccessToken != "sil_live_oauth_x" || tok.TokenType != "Bearer" || tok.ClientID != "c" || tok.TeamID != "p" {
		t.Errorf("unexpected token response: %+v", tok)
	}

	// With codeVerifier set, it must be included.
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "t", "token_type": "Bearer", "client_id": "c", "team_id": "p",
			"scopes": []string{}, "plan": testPlan(),
		})
	}))
	defer ts2.Close()

	_, err = seeitlive.ExchangeCode(context.Background(), seeitlive.ExchangeCodeParams{
		Code: "code123", ClientID: "c", ClientSecret: "s", RedirectURI: "https://x/cb",
		CodeVerifier: "verifier", BaseURL: ts2.URL + "/v1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var body2 map[string]interface{}
	_ = json.Unmarshal(gotBody, &body2)
	if body2["code_verifier"] != "verifier" {
		t.Errorf("code_verifier = %v, want verifier", body2["code_verifier"])
	}
}

// (d) HMAC request signing: headers present when SigningSecret is
// configured, absent otherwise.
func TestRequestSigningHeaders(t *testing.T) {
	var gotSig, gotTS string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-SeeItLive-Signature")
		gotTS = r.Header.Get("X-SeeItLive-Timestamp")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "s1", "status": "pending", "inviteCode": "abc", "inviteUrl": "u", "agentViewUrl": "a", "expiresAt": nil,
		})
	}))
	defer ts.Close()

	signedClient, err := seeitlive.New(seeitlive.Config{
		APIKey:        testAPIKey,
		BaseURL:       ts.URL + "/v1",
		SigningSecret: "wh_secret_" + strings.Repeat("b", 32),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := signedClient.Sessions.Create(context.Background(), seeitlive.CreateSessionParams{Channel: seeitlive.ChannelLink}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSig == "" || !strings.HasPrefix(gotSig, "sha256=") {
		t.Errorf("X-SeeItLive-Signature = %q, want a sha256=<hex> value", gotSig)
	}
	if gotTS == "" {
		t.Error("X-SeeItLive-Timestamp must be set when SigningSecret is configured")
	}

	// Reset and verify no signing headers are sent when SigningSecret is unset.
	gotSig, gotTS = "", ""
	unsignedClient, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := unsignedClient.Sessions.Create(context.Background(), seeitlive.CreateSessionParams{Channel: seeitlive.ChannelLink}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSig != "" {
		t.Errorf("X-SeeItLive-Signature = %q, want empty when SigningSecret is unset", gotSig)
	}
	if gotTS != "" {
		t.Errorf("X-SeeItLive-Timestamp = %q, want empty when SigningSecret is unset", gotTS)
	}
}

func TestSessionsGetProtection(t *testing.T) {
	var gotMethod, gotPath string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"type":             "password",
			"password":         "K7QMX2RT",
			"code":             nil,
			"secondsRemaining": nil,
			"locked":           false,
			"failedAttempts":   0,
		})
	}))
	defer ts.Close()

	client, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p, err := client.Sessions.GetProtection(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/v1/sessions/s1/protection" {
		t.Errorf("path = %q, want /v1/sessions/s1/protection", gotPath)
	}
	if p.Type != "password" {
		t.Errorf("type = %q, want password", p.Type)
	}
	if p.Password == nil || *p.Password != "K7QMX2RT" {
		t.Errorf("password = %v, want K7QMX2RT", p.Password)
	}
	if p.Code != nil {
		t.Errorf("code = %v, want nil", p.Code)
	}
}

func TestSessionsSetProtection(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "password", "password": "K7QMX2RT", "code": nil,
			"secondsRemaining": nil, "locked": false, "failedAttempts": 0,
		})
	}))
	defer ts.Close()

	client, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := client.Sessions.SetProtection(context.Background(), "s1", seeitlive.SetProtectionParams{
		Protection: "password", ProtectionSecret: "K7QMX2RT",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/v1/sessions/s1/protection" {
		t.Errorf("path = %q, want /v1/sessions/s1/protection", gotPath)
	}
	if gotBody["protectionSecret"] != "K7QMX2RT" {
		t.Errorf("protectionSecret = %v, want K7QMX2RT", gotBody["protectionSecret"])
	}
}

// omitempty is load-bearing: the API generates a secret only when the field is
// ABSENT, so an empty ProtectionSecret must not serialize as "".
func TestSessionsSetProtectionOmitsEmptySecret(t *testing.T) {
	var gotBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "password", "password": "GENERATED", "code": nil,
			"secondsRemaining": nil, "locked": false, "failedAttempts": 0,
		})
	}))
	defer ts.Close()

	client, _ := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
	if _, err := client.Sessions.SetProtection(context.Background(), "s1", seeitlive.SetProtectionParams{
		Protection: "password",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, present := gotBody["protectionSecret"]; present {
		t.Errorf("protectionSecret present = %v, want absent", gotBody["protectionSecret"])
	}
	if gotBody["protection"] != "password" {
		t.Errorf("protection = %v, want password", gotBody["protection"])
	}
}

// (l) Sessions.Get reads a stored session, including its invitation.
func TestSessionsGet(t *testing.T) {
	var gotMethod, gotPath string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "s1", "teamId": "t1", "env": "live", "status": "active",
			"note": nil, "metadata": map[string]string{"ticket": "42"},
			"startedAt": "2026-01-01T00:00:00Z", "endedAt": nil, "createdAt": "2026-01-01T00:00:00Z",
			"invitation": map[string]interface{}{
				"code": "abc123", "channel": "sms", "recipient": "+15145551234",
				"expiresAt": "2026-01-01T01:00:00Z", "openedAt": nil,
			},
		})
	}))
	defer ts.Close()

	client, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s, err := client.Sessions.Get(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/v1/sessions/s1" {
		t.Errorf("path = %q, want /v1/sessions/s1", gotPath)
	}
	if s.TeamID != "t1" {
		t.Errorf("teamId = %q, want t1", s.TeamID)
	}
	if s.Invitation == nil || s.Invitation.Code != "abc123" {
		t.Fatalf("invitation = %+v, want code abc123", s.Invitation)
	}
	if s.Invitation.Channel != seeitlive.ChannelSMS {
		t.Errorf("channel = %q, want sms", s.Invitation.Channel)
	}
	if s.Metadata["ticket"] != "42" {
		t.Errorf("metadata[ticket] = %q, want 42", s.Metadata["ticket"])
	}
}

// (m) Sessions.ResetProtection clears a lockout, rotating only when asked.
func TestSessionsResetProtection(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotBody = map[string]interface{}{}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "password", "password": "K7QMX2RT", "code": nil,
			"secondsRemaining": nil, "locked": false, "failedAttempts": 0,
		})
	}))
	defer ts.Close()

	client, _ := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})

	rotate := true
	p, err := client.Sessions.ResetProtection(context.Background(), "s1", seeitlive.ResetProtectionParams{Rotate: &rotate})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v1/sessions/s1/protection/reset" {
		t.Errorf("path = %q, want /v1/sessions/s1/protection/reset", gotPath)
	}
	if gotBody["rotate"] != true {
		t.Errorf("rotate = %v, want true", gotBody["rotate"])
	}
	if p.Locked {
		t.Error("locked = true, want false")
	}

	// Clearing a lockout is the common case and must not rotate by accident.
	if _, err := client.Sessions.ResetProtection(context.Background(), "s1", seeitlive.ResetProtectionParams{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := gotBody["rotate"]; present {
		t.Errorf("rotate present = %v, want absent", gotBody["rotate"])
	}
}

// (n) Sessions.CreateAgentToken mints a viewer token, labelling it when asked.
func TestSessionsCreateAgentToken(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotBody = map[string]interface{}{}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "vw1", "token": "tok", "agentViewUrl": "https://app.seeitlive.io/v/vw1#tok",
		})
	}))
	defer ts.Close()

	client, _ := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})

	tok, err := client.Sessions.CreateAgentToken(context.Background(), "s1", seeitlive.CreateAgentTokenParams{Label: "Odoo — crm.lead 42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v1/sessions/s1/agent-token" {
		t.Errorf("path = %q, want /v1/sessions/s1/agent-token", gotPath)
	}
	if gotBody["label"] != "Odoo — crm.lead 42" {
		t.Errorf("label = %v, want the supplied label", gotBody["label"])
	}
	if tok.AgentViewURL != "https://app.seeitlive.io/v/vw1#tok" {
		t.Errorf("agentViewUrl = %q, want the minted viewer URL", tok.AgentViewURL)
	}

	if _, err := client.Sessions.CreateAgentToken(context.Background(), "s1", seeitlive.CreateAgentTokenParams{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := gotBody["label"]; present {
		t.Errorf("label present = %v, want absent so the API applies its own default", gotBody["label"])
	}
}

// (o) Snapshots.List returns a session's snapshot metadata.
func TestSnapshotsList(t *testing.T) {
	var gotPath string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "sn1", "width": 1280, "height": 720, "capturedAt": "2026-01-01T00:00:00Z", "url": "/api/v1/snapshots/sn1"},
		})
	}))
	defer ts.Close()

	client, _ := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})

	list, err := client.Snapshots.List(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/v1/sessions/s1/snapshots" {
		t.Errorf("path = %q, want /v1/sessions/s1/snapshots", gotPath)
	}
	if len(list) != 1 || list[0].ID != "sn1" {
		t.Fatalf("list = %+v, want one snapshot sn1", list)
	}
	if list[0].Width == nil || *list[0].Width != 1280 {
		t.Errorf("width = %v, want 1280", list[0].Width)
	}
}

// (p) Snapshots.Get asks for base64 JSON rather than raw PNG bytes, and signs
// the query string along with the path (the server signs path+query).
func TestSnapshotsGetUsesBase64Query(t *testing.T) {
	var gotPath, gotQuery, gotSig, gotTS string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		gotSig, gotTS = r.Header.Get("X-SeeItLive-Signature"), r.Header.Get("X-SeeItLive-Timestamp")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "sn1", "contentType": "image/png", "width": 1280, "height": 720,
			"capturedAt": "2026-01-01T00:00:00Z", "data": "aVZCT1J3MEs=",
		})
	}))
	defer ts.Close()

	secret := "whsec_" + strings.Repeat("b", 32)
	client, _ := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1", SigningSecret: secret})

	img, err := client.Snapshots.Get(context.Background(), "sn1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/v1/snapshots/sn1" {
		t.Errorf("path = %q, want /v1/snapshots/sn1", gotPath)
	}
	if gotQuery != "format=base64" {
		t.Errorf("query = %q, want format=base64", gotQuery)
	}
	if img.Data != "aVZCT1J3MEs=" {
		t.Errorf("data = %q, want the base64 payload", img.Data)
	}
	// Recompute the documented base string independently rather than reaching
	// into the SDK's own signer, so a change to either side shows up here.
	emptyBody := sha256.Sum256(nil)
	base := gotTS + ".GET./snapshots/sn1?format=base64." + hex.EncodeToString(emptyBody[:])
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(base))
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if gotSig != want {
		t.Errorf("signature = %q, want %q (signed over path INCLUDING query)", gotSig, want)
	}
}

func TestSessionsReshare(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"expiresAt": "2026-01-02T00:00:00Z", "inviteShortUrl": "https://sitl.cc/s/aB3xK9",
			"resend": "sent", "rotated": true,
		})
	}))
	defer ts.Close()

	client, err := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	smsConsent := true
	res, err := client.Sessions.Reshare(context.Background(), "s1", seeitlive.ReshareParams{
		Mode: "new", Channel: seeitlive.ChannelSMS, Recipient: "+15145550199", SMSConsent: &smsConsent,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v1/sessions/s1/reshare" {
		t.Errorf("path = %q, want /v1/sessions/s1/reshare", gotPath)
	}
	if !res.Rotated {
		t.Errorf("rotated = %v, want true", res.Rotated)
	}
	if res.Resend != "sent" {
		t.Errorf("resend = %q, want sent", res.Resend)
	}
	if res.InviteShortURL == nil || *res.InviteShortURL != "https://sitl.cc/s/aB3xK9" {
		t.Errorf("inviteShortUrl = %v, want https://sitl.cc/s/aB3xK9", res.InviteShortURL)
	}
	want := map[string]interface{}{
		"mode": "new", "channel": "sms", "recipient": "+15145550199", "smsConsent": true,
	}
	if len(gotBody) != len(want) {
		t.Errorf("body = %v, want %v", gotBody, want)
	}
	for k, v := range want {
		if gotBody[k] != v {
			t.Errorf("body[%q] = %v, want %v", k, gotBody[k], v)
		}
	}
}

// Omitted options must be ABSENT from the body, not sent as null — absent
// means "reuse what was recorded"; null is a different instruction.
func TestSessionsReshareOmitsUnsetOptions(t *testing.T) {
	var gotBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"expiresAt": "2026-01-02T00:00:00Z", "inviteShortUrl": nil,
			"resend": "not_requested", "rotated": false,
		})
	}))
	defer ts.Close()

	client, _ := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
	res, err := client.Sessions.Reshare(context.Background(), "s1", seeitlive.ReshareParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.InviteShortURL != nil {
		t.Errorf("inviteShortUrl = %v, want nil", res.InviteShortURL)
	}
	if len(gotBody) != 0 {
		t.Errorf("body = %v, want empty object", gotBody)
	}
}

func TestSessionsReshareEscapesID(t *testing.T) {
	var gotPath string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"expiresAt": "2026-01-02T00:00:00Z", "inviteShortUrl": nil,
			"resend": "not_requested", "rotated": false,
		})
	}))
	defer ts.Close()

	client, _ := seeitlive.New(seeitlive.Config{APIKey: testAPIKey, BaseURL: ts.URL + "/v1"})
	if _, err := client.Sessions.Reshare(context.Background(), "sess/../evil", seeitlive.ReshareParams{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/v1/sessions/sess%2F..%2Fevil/reshare" {
		t.Errorf("path = %q, want /v1/sessions/sess%%2F..%%2Fevil/reshare", gotPath)
	}
}
