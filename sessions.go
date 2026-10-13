package seeitlive

import (
	"context"
	"net/url"
)

// Sessions groups the session-related API calls.
type Sessions struct {
	http *httpClient
}

// Create starts a new session invite. POST /sessions.
func (s *Sessions) Create(ctx context.Context, params CreateSessionParams) (*Session, error) {
	var out Session
	if err := s.http.request(ctx, "/sessions", requestOptions{method: "POST", body: params}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get reads a stored session, including how its invitation was delivered.
// GET /sessions/{id}.
func (s *Sessions) Get(ctx context.Context, id string) (*SessionDetail, error) {
	var out SessionDetail
	if err := s.http.request(ctx, "/sessions/"+url.PathEscape(id), requestOptions{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Status fetches the current status of a session. GET /sessions/{id}/status.
func (s *Sessions) Status(ctx context.Context, id string) (*SessionStatus, error) {
	var out SessionStatus
	path := "/sessions/" + url.PathEscape(id) + "/status"
	if err := s.http.request(ctx, path, requestOptions{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetProtection reads the secret to give the customer: the password, or the
// rotating code of the moment. GET /sessions/{id}/protection.
func (s *Sessions) GetProtection(ctx context.Context, id string) (*ProtectionInfo, error) {
	var out ProtectionInfo
	path := "/sessions/" + url.PathEscape(id) + "/protection"
	if err := s.http.request(ctx, path, requestOptions{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetProtection changes a session's guest challenge after creation, including
// adding one to a session created without any. PUT /sessions/{id}/protection.
func (s *Sessions) SetProtection(ctx context.Context, id string, params SetProtectionParams) (*ProtectionInfo, error) {
	var out ProtectionInfo
	path := "/sessions/" + url.PathEscape(id) + "/protection"
	if err := s.http.request(ctx, path, requestOptions{method: "PUT", body: params}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Reshare revives an expired invitation. Mode "revive" (the default) extends
// the SAME link and code in place, so a message the customer already has
// starts working again; Mode "new" issues a fresh link and kills the old
// one.
//
// A session created without a short link can be extended but not re-sent —
// the plaintext token survives only in the short link's encrypted fragment.
// Resend reports which happened. POST /sessions/{id}/reshare.
func (s *Sessions) Reshare(ctx context.Context, id string, params ReshareParams) (*ReshareResult, error) {
	var out ReshareResult
	path := "/sessions/" + url.PathEscape(id) + "/reshare"
	if err := s.http.request(ctx, path, requestOptions{method: "POST", body: params}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ResetProtection clears a guest lockout, optionally rotating the secret.
// POST /sessions/{id}/protection/reset.
func (s *Sessions) ResetProtection(ctx context.Context, id string, params ResetProtectionParams) (*ProtectionInfo, error) {
	var out ProtectionInfo
	path := "/sessions/" + url.PathEscape(id) + "/protection/reset"
	if err := s.http.request(ctx, path, requestOptions{method: "POST", body: params}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateAgentToken mints a fresh agent-view token so an embedded viewer can
// watch the session. POST /sessions/{id}/agent-token.
func (s *Sessions) CreateAgentToken(ctx context.Context, id string, params CreateAgentTokenParams) (*AgentToken, error) {
	var out AgentToken
	path := "/sessions/" + url.PathEscape(id) + "/agent-token"
	if err := s.http.request(ctx, path, requestOptions{method: "POST", body: params}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
