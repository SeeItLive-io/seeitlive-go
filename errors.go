package seeitlive

// APIError is the base error type for all non-2xx responses from the
// SeeItLive API. Status-specific errors below embed APIError so that
// errors.As works both against the concrete subtype and against *APIError
// itself.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string { return e.Message }

// ValidationError maps to HTTP 400. Field is populated when the API
// response body includes a "field" property.
type ValidationError struct {
	APIError
	Field string
}

// Unwrap lets errors.As(err, &apiErr) reach the embedded base APIError,
// mirroring `instanceof SeeItLiveError` in the Node reference SDK.
func (e *ValidationError) Unwrap() error { return &e.APIError }

// AuthenticationError maps to HTTP 401.
type AuthenticationError struct {
	APIError
}

func (e *AuthenticationError) Unwrap() error { return &e.APIError }

// PermissionError maps to HTTP 403.
type PermissionError struct {
	APIError
}

func (e *PermissionError) Unwrap() error { return &e.APIError }

// NotFoundError maps to HTTP 404.
type NotFoundError struct {
	APIError
}

func (e *NotFoundError) Unwrap() error { return &e.APIError }

// RateLimitError maps to HTTP 429. RetryAfter is a pointer because the
// "Retry-After" header may legitimately be 0 (retry immediately) — nil
// means the header was absent or unparsable, distinct from a genuine 0.
type RateLimitError struct {
	APIError
	RetryAfter *int
}

func (e *RateLimitError) Unwrap() error { return &e.APIError }
