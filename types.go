package seeitlive

// Env is the inferred (or explicitly configured) operating environment.
type Env string

const (
	EnvLive Env = "live"
	EnvTest Env = "test"
)

// InviteChannel selects how a session invite is delivered.
type InviteChannel string

const (
	ChannelSMS   InviteChannel = "sms"
	ChannelEmail InviteChannel = "email"
	ChannelLink  InviteChannel = "link"
)

// Config configures a new Client. Exactly one of APIKey or OAuthToken must
// be set.
type Config struct {
	APIKey     string
	OAuthToken string
	BaseURL    string // default: https://api.seeitlive.io/v1
	Env        Env    // optional override; otherwise inferred from token prefix
	// SigningSecret, when set, causes every request to be signed with
	// X-SeeItLive-Signature / X-SeeItLive-Timestamp headers (HMAC-SHA256).
	// Leave empty (the default) to send unsigned requests.
	SigningSecret string
}

// CreateSessionParams are the parameters for Sessions.Create. The SDK does
// NOT send a teamId — the team is implicit from the API key / OAuth
// token. The API requires RecipientPhone for the "sms" channel and
// RecipientEmail for "email"; the SDK forwards whatever is given and lets
// the API validate.
type CreateSessionParams struct {
	Channel        InviteChannel `json:"channel"`
	RecipientPhone string        `json:"recipientPhone,omitempty"`
	RecipientEmail string        `json:"recipientEmail,omitempty"`
	Note           string        `json:"note,omitempty"`
	TTLMinutes     *int          `json:"ttlMinutes,omitempty"`
	// SMSConsent attests the recipient agreed to receive a text. The API
	// requires it to be true for the "sms" channel (carrier fees may apply).
	SMSConsent *bool `json:"smsConsent,omitempty"`
	// Metadata is an optional flat map of string keys to string values
	// attached to the session; omitted from the request when empty.
	Metadata map[string]string `json:"metadata,omitempty"`
	// ShortLink requests that the API also mint a sitl.cc/s/<code> short
	// link for the session invite.
	ShortLink *bool `json:"shortLink,omitempty"`
	// Protection requires the guest to answer a challenge before sharing
	// camera: "password" (a static secret) or "totp" (a rotating 6-digit
	// code the agent reads aloud). Empty means no protection.
	Protection string `json:"protection,omitempty"`
	// ProtectionSecret is the password or base32 TOTP seed. Leave empty and
	// the API generates one, returned once on the Session.
	ProtectionSecret string `json:"protectionSecret,omitempty"`
}

// Session is the response from Sessions.Create.
type Session struct {
	ID                   string            `json:"id"`
	Status               string            `json:"status"`
	InviteCode           string            `json:"inviteCode"`
	InviteURL            string            `json:"inviteUrl"`
	InviteShortURL       *string           `json:"inviteShortUrl"`
	AgentViewURL         string            `json:"agentViewUrl"`
	Metadata             map[string]string `json:"metadata,omitempty"`
	ExpiresAt            *string           `json:"expiresAt"`
	Protection           string            `json:"protection"`
	ProtectionSecret     *string           `json:"protectionSecret"`
	ProtectionOtpauthURL *string           `json:"protectionOtpauthUrl"`
}

// SessionStatus is the response from Sessions.Status.
type SessionStatus struct {
	ID            string  `json:"id"`
	Status        string  `json:"status"`
	StartedAt     *string `json:"startedAt"`
	EndedAt       *string `json:"endedAt"`
	ProducerCount int     `json:"producerCount"`
	HasVideo      bool    `json:"hasVideo"`
}

// ProtectionInfo is the agent's view of a session's guest challenge. For "totp"
// Code carries the CURRENT code, never the seed — the seed leaves the server
// only once, in the create response.
type ProtectionInfo struct {
	Type             string  `json:"type"`
	Password         *string `json:"password"`
	Code             *string `json:"code"`
	SecondsRemaining *int    `json:"secondsRemaining"`
	Locked           bool    `json:"locked"`
	FailedAttempts   int     `json:"failedAttempts"`
}

// SetProtectionParams changes a session's guest challenge after creation. Leave
// ProtectionSecret empty and the API generates one; Protection "none" clears the
// challenge and any lockout with it.
type SetProtectionParams struct {
	Protection       string `json:"protection"`
	ProtectionSecret string `json:"protectionSecret,omitempty"`
}

// ReshareParams are the options for reviving (or replacing) an expired
// invitation. Every field is optional; an omitted field is absent from the
// request body and means "reuse what the invitation already recorded".
type ReshareParams struct {
	// Mode "revive" (the default) extends the existing link in place; "new"
	// issues a fresh one and kills the old.
	Mode string `json:"mode,omitempty"`
	// Channel changes how the customer is reached. Omit to reuse the
	// recorded channel.
	Channel InviteChannel `json:"channel,omitempty"`
	// Recipient changes the phone number or email address. Omit to reuse
	// the recorded one.
	Recipient string `json:"recipient,omitempty"`
	// SMSConsent is required and must be true when Channel is "sms".
	SMSConsent *bool `json:"smsConsent,omitempty"`
}

// ReshareResult reports how a revived invitation was (or wasn't) delivered.
type ReshareResult struct {
	ExpiresAt string `json:"expiresAt"`
	// InviteShortURL is nil when the session has no short link — it can be
	// extended but not re-sent.
	InviteShortURL *string `json:"inviteShortUrl"`
	Resend         string  `json:"resend"`
	// Rotated is true when a new code/link was issued (Mode "new").
	Rotated bool `json:"rotated"`
}

// SessionInvitation is a session's invitation as the agent side sees it —
// never the token hash and never the guest secret.
type SessionInvitation struct {
	Code      string        `json:"code"`
	Channel   InviteChannel `json:"channel"`
	Recipient *string       `json:"recipient"`
	ExpiresAt string        `json:"expiresAt"`
	OpenedAt  *string       `json:"openedAt"`
}

// SessionDetail is a stored session read back by id (Sessions.Get), unlike
// Session which is the one-time create response.
type SessionDetail struct {
	ID                string             `json:"id"`
	TeamID            string             `json:"teamId"`
	Env               Env                `json:"env"`
	Status            string             `json:"status"`
	CreatedByClientID *string            `json:"createdByClientId"`
	CreatedByAPIKeyID *string            `json:"createdByApiKeyId"`
	Note              *string            `json:"note"`
	Metadata          map[string]string  `json:"metadata"`
	StartedAt         *string            `json:"startedAt"`
	EndedAt           *string            `json:"endedAt"`
	CreatedAt         string             `json:"createdAt"`
	Invitation        *SessionInvitation `json:"invitation"`
}

// ResetProtectionParams clears a guest lockout. Rotation is opt-in: a lockout
// is usually the customer mistyping, and a new secret would just make the agent
// read it out again. Leave Rotate nil to keep the existing secret.
type ResetProtectionParams struct {
	Rotate *bool `json:"rotate,omitempty"`
}

// CreateAgentTokenParams labels the minted viewer in the dashboard's list
// (e.g. "Odoo — crm.lead 42"). Leave Label empty and the API applies its own
// default, branched by principal.
type CreateAgentTokenParams struct {
	Label string `json:"label,omitempty"`
}

// AgentToken is a freshly minted agent-view grant. Tokens expire after 120
// minutes; mint another to re-open a viewer.
type AgentToken struct {
	Code         string `json:"code"`
	Token        string `json:"token"`
	AgentViewURL string `json:"agentViewUrl"`
}

// SnapshotSummary is one entry of Snapshots.List. URL is the API path the
// bytes are served from.
type SnapshotSummary struct {
	ID         string `json:"id"`
	Width      *int   `json:"width"`
	Height     *int   `json:"height"`
	CapturedAt string `json:"capturedAt"`
	URL        string `json:"url"`
}

// SnapshotImage is one snapshot fetched by Snapshots.Get. Data is the
// base64-encoded PNG.
type SnapshotImage struct {
	ID          string `json:"id"`
	ContentType string `json:"contentType"`
	Width       *int   `json:"width"`
	Height      *int   `json:"height"`
	CapturedAt  string `json:"capturedAt"`
	Data        string `json:"data"`
}

// Plan describes the plan associated with a client/team.
type Plan struct {
	PlanKey         string `json:"planKey"`
	PlanName        string `json:"planName"`
	SessionLimit    *int   `json:"sessionLimit"`
	SeatLimit       *int   `json:"seatLimit"`
	ConcurrentLimit *int   `json:"concurrentLimit"`
}

// TokenInfo is the response from OAuth.Verify.
type TokenInfo struct {
	Valid     bool     `json:"valid"`
	ClientID  string   `json:"clientId"`
	TeamID    string   `json:"teamId"`
	Scopes    []string `json:"scopes"`
	Plan      Plan     `json:"plan"`
	ExpiresAt *string  `json:"expiresAt"`
}

// ExchangeCodeParams are the parameters for the standalone ExchangeCode
// function.
type ExchangeCodeParams struct {
	Code         string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	CodeVerifier string // optional; omitted from the request when empty
	BaseURL      string // default: https://api.seeitlive.io/v1
}

// TokenResponse is the response from ExchangeCode.
type TokenResponse struct {
	AccessToken string   `json:"accessToken"`
	TokenType   string   `json:"tokenType"`
	ClientID    string   `json:"clientId"`
	TeamID      string   `json:"teamId"`
	Scopes      []string `json:"scopes"`
	Plan        Plan     `json:"plan"`
}
