// Package seeitlive is the official Go client SDK for the SeeItLive API.
package seeitlive

import (
	"errors"
	"strings"
)

const defaultBaseURL = "https://api.seeitlive.io/v1"

// Client is the SeeItLive API client. Construct with New.
type Client struct {
	Env       Env
	Sessions  *Sessions
	Snapshots *Snapshots
	OAuth     *OAuth
}

// New constructs a Client. Exactly one of Config.APIKey or Config.OAuthToken
// must be set; providing neither, or both, is an error.
//
// Env is inferred from the token prefix (a token starting "sil_test_" ->
// "test", else "live") unless Config.Env is set explicitly. BaseURL
// defaults to https://api.seeitlive.io/v1 (trailing slashes trimmed).
func New(cfg Config) (*Client, error) {
	if cfg.APIKey == "" && cfg.OAuthToken == "" {
		return nil, errors.New("seeitlive: either APIKey or OAuthToken must be provided")
	}
	if cfg.APIKey != "" && cfg.OAuthToken != "" {
		return nil, errors.New("seeitlive: provide only one of APIKey or OAuthToken")
	}

	token := cfg.APIKey
	if token == "" {
		token = cfg.OAuthToken
	}

	env := cfg.Env
	if env == "" {
		if strings.HasPrefix(token, "sil_test_") {
			env = EnvTest
		} else {
			env = EnvLive
		}
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	h := newHTTPClient(baseURL, "Bearer "+token, cfg.SigningSecret)

	return &Client{
		Env:       env,
		Sessions:  &Sessions{http: h},
		Snapshots: &Snapshots{http: h},
		OAuth:     &OAuth{http: h},
	}, nil
}
