package ibs

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// hmacFieldSeparator is inserted between body, apiKey, timestamp, and nonce
// when building the HMAC payload. Using 0x1F (ASCII unit separator) makes the
// boundary between fields unambiguous so two distinct tuples can never produce
// the same payload by shifting bytes across boundaries.
const hmacFieldSeparator = 0x1F

// Config holds the required configuration for connecting to the IBS API.
type Config struct {
	// APIURL is the base URL of the IBS API (e.g. "https://api.ibs.example.com").
	APIURL string
	// APIKey is the API key used for authentication and signing.
	APIKey string
	// SecretKey is the base64-encoded secret key used for HMAC signing.
	SecretKey string
}

// Option is a functional option applied during [Configure].
type Option func(*globalConfig)

// WithUserAgent sets a custom User-Agent header for all API requests.
func WithUserAgent(ua string) Option {
	return func(g *globalConfig) {
		g.userAgent = ua
	}
}

// WithLogger sets a custom slog.Logger for the SDK. Pass nil to disable logging.
func WithLogger(l *slog.Logger) Option {
	return func(g *globalConfig) {
		g.logger = l
	}
}

// WithHTTPClient sets a custom http.Client for all API requests.
func WithHTTPClient(hc *http.Client) Option {
	return func(g *globalConfig) {
		g.httpClient = hc
	}
}

// ---------- global state ----------

type globalConfig struct {
	apiURL     string
	apiKey     string
	secretKey  string
	userAgent  string
	logger     *slog.Logger
	httpClient *http.Client
}

var (
	global     atomic.Pointer[globalConfig]
	globalOnce sync.Once

	// ErrNotConfigured is returned when New is called before Configure.
	ErrNotConfigured = errors.New("ibs: sdk not configured — call ibs.Configure() first")
)

// Configure initialises the IBS SDK with the given credentials and options.
// It must be called once at program startup before any call to [New].
// Calling Configure more than once will panic.
func Configure(cfg Config, opts ...Option) {
	called := false
	globalOnce.Do(func() {
		called = true
		g := &globalConfig{
			apiURL:    cfg.APIURL,
			apiKey:    cfg.APIKey,
			secretKey: cfg.SecretKey,
			userAgent: "IBS-SDK/1.0",
			logger:    slog.Default(),
			httpClient: &http.Client{
				Timeout: time.Minute,
			},
		}
		for _, opt := range opts {
			opt(g)
		}
		global.Store(g)
	})
	if !called {
		panic("ibs: Configure() must only be called once")
	}
}

// Configured reports whether [Configure] has already been called.
func Configured() bool {
	return global.Load() != nil
}

// ---------- Client ----------

// Client is the IBS card provider API client.
// Create one with [New] after calling [Configure].
type Client struct {
	userID string
	cardID string

	// shared (read-only after Configure)
	g *globalConfig
}

// New creates a new IBS Client scoped to the given user and card.
// The SDK must have been initialised with [Configure] before calling New.
//
//	client := ibs.New("user-123", "card-456")
//	client := ibs.New("user-123", "")          // user-only context
//	client := ibs.New("", "")                   // bare client
func New(userID, cardID string) *Client {
	g := global.Load()
	if g == nil {
		panic(ErrNotConfigured)
	}
	return &Client{
		userID: userID,
		cardID: cardID,
		g:      g,
	}
}

// WithUser returns a shallow copy of the client with a different user ID.
func (c *Client) WithUser(userID string) *Client {
	return &Client{userID: userID, cardID: c.cardID, g: c.g}
}

// WithCard returns a shallow copy of the client with a different card ID.
func (c *Client) WithCard(cardID string) *Client {
	return &Client{userID: c.userID, cardID: cardID, g: c.g}
}

// WithUserAndCard returns a shallow copy of the client with different user and card IDs.
func (c *Client) WithUserAndCard(userID, cardID string) *Client {
	return &Client{userID: userID, cardID: cardID, g: c.g}
}

// UserID returns the current user ID context.
func (c *Client) UserID() string { return c.userID }

// CardID returns the current card ID context.
func (c *Client) CardID() string { return c.cardID }

// ---------- internal helpers ----------

func newNonce() (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("ibs: generate nonce: %w", err)
	}
	return hex.EncodeToString(nonce), nil
}

// sign produces the HMAC-SHA256 signature used by the IBS API. Fields are
// joined by hmacFieldSeparator so the boundary between body, apiKey,
// timestamp, and nonce is unambiguous.
func (c *Client) sign(body []byte, timestamp, nonce string) (string, error) {
	ds, err := base64.StdEncoding.DecodeString(c.g.secretKey)
	if err != nil {
		return "", fmt.Errorf("ibs: failed to decode secret key: %w", err)
	}

	message := make([]byte, 0, len(body)+len(c.g.apiKey)+len(timestamp)+len(nonce)+3)
	message = append(message, body...)
	message = append(message, hmacFieldSeparator)
	message = append(message, c.g.apiKey...)
	message = append(message, hmacFieldSeparator)
	message = append(message, timestamp...)
	message = append(message, hmacFieldSeparator)
	message = append(message, nonce...)

	h := hmac.New(sha256.New, ds)
	h.Write(message)

	return hex.EncodeToString(h.Sum(nil)), nil
}

// APIError is returned when the IBS API responds with status=false.
type APIError struct {
	StatusCode int
	Body       string
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ibs: api error (HTTP %d): %s", e.StatusCode, e.Message)
}

// IsAPIError reports whether the error is an IBS API-level error and returns its details.
func IsAPIError(err error) (statusCode int, message string, ok bool) {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.StatusCode, ae.Message, true
	}
	return 0, "", false
}

// Sentinel errors for API responses that callers commonly want to branch on.
// Use errors.Is to test against these — they are exposed as wrappable values
// rather than exported APIError instances so the StatusCode/Body fields of
// the underlying APIError remain available via errors.As.
var (
	// ErrDuplicateIdempotencyKey is returned when the server rejects a
	// monetary request because the X-Idempotency-Key has already been
	// processed. The original operation is NOT re-executed; if you need
	// the prior result, fetch it by transaction ID from your own records.
	// This corresponds to HTTP 409 from /card/balance/add and /card/balance/dec.
	ErrDuplicateIdempotencyKey = errors.New("ibs: duplicate idempotency key")

	// ErrPayloadTooLarge is returned when the request body exceeds the
	// server's body-size limit (currently 1 MiB at the time of writing).
	// HTTP 413. This shouldn't happen in normal SDK usage; if it does,
	// inspect what you're sending in the request body.
	ErrPayloadTooLarge = errors.New("ibs: request payload too large")

	// ErrRateLimited is returned when the server throttles the request
	// (HTTP 429). The Retry-After header, if present, is surfaced via the
	// underlying APIError.Body. The /v1/card/* endpoints are not currently
	// rate-limited, so this is mostly future-proofing.
	ErrRateLimited = errors.New("ibs: rate limited")
)

// IsDuplicateIdempotencyKey reports whether err indicates the server saw
// the same X-Idempotency-Key for a prior request. Convenience wrapper around
// errors.Is(err, ErrDuplicateIdempotencyKey) for callers that prefer it.
func IsDuplicateIdempotencyKey(err error) bool {
	return errors.Is(err, ErrDuplicateIdempotencyKey)
}

// classifyStatus maps an HTTP status to a sentinel error if one applies, or
// nil otherwise. Keep in sync with the server's well-known response codes
// in ibs/middleware/{idempotency,bodylimit,ratelimit}.
func classifyStatus(status int) error {
	switch status {
	case http.StatusConflict:
		return ErrDuplicateIdempotencyKey
	case http.StatusRequestEntityTooLarge:
		return ErrPayloadTooLarge
	case http.StatusTooManyRequests:
		return ErrRateLimited
	}
	return nil
}

// statusReason extracts a short human reason from the response body for use
// in wrapped sentinel errors. Falls back to "HTTP <code>" if the body isn't
// the expected envelope.
func statusReason(body []byte, status int) string {
	var env apiStatusEnvelope
	if json.Valid(body) {
		if err := json.Unmarshal(body, &env); err == nil && env.Error != "" {
			return env.Error
		}
	}
	return fmt.Sprintf("HTTP %d", status)
}

// apiStatusEnvelope is used to check the top-level status/error fields in every response.
type apiStatusEnvelope struct {
	Status bool   `json:"status"`
	Error  string `json:"error,omitempty"`
}

// IdempotencyKeyHeader is the header used by the IBS API to deduplicate
// monetary requests. Sending the same key twice within the server-side
// retention window returns 409 Conflict rather than re-executing the
// operation. Required on /balance/add and /balance/dec.
const IdempotencyKeyHeader = "X-Idempotency-Key"

// newIdempotencyKey returns a 32-hex-char random token. Callers that need
// retry-safety should reuse the same key across retries of the *same*
// logical operation.
func newIdempotencyKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("ibs: generate idempotency key: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// requestAPI performs an authenticated (or unauthenticated) request to the IBS API
// and returns the raw response body bytes. It checks the top-level "status" field
// and returns an [APIError] when it is false.
func (c *Client) requestAPI(method, endpoint string, body map[string]any, auth bool) ([]byte, error) {
	return c.requestAPIWithIdempotency(method, endpoint, body, auth, "")
}

// requestAPIWithIdempotency is the same as requestAPI but also sets
// X-Idempotency-Key when idempotencyKey is non-empty.
func (c *Client) requestAPIWithIdempotency(method, endpoint string, body map[string]any, auth bool, idempotencyKey string) ([]byte, error) {
	reqURL := c.g.apiURL + endpoint

	var bodyJSON []byte
	var req *http.Request
	var err error

	if body != nil {
		bodyJSON, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("ibs: marshal request body: %w", err)
		}
		req, err = http.NewRequest(method, reqURL, bytes.NewReader(bodyJSON))
	} else {
		req, err = http.NewRequest(method, reqURL, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("ibs: create request: %w", err)
	}

	req.Header.Set("User-Agent", c.g.userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if idempotencyKey != "" {
		req.Header.Set(IdempotencyKeyHeader, idempotencyKey)
	}

	if auth {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		nonce, err := newNonce()
		if err != nil {
			return nil, err
		}
		signature, err := c.sign(bodyJSON, timestamp, nonce)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Api-Key", c.g.apiKey)
		req.Header.Set("X-Signature", signature)
		req.Header.Set("X-Timestamp", timestamp)
		req.Header.Set("X-Nonce", nonce)
	}

	resp, err := c.g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ibs: http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ibs: read response body: %w", err)
	}

	// Map HTTP status codes that carry SDK-actionable semantics to typed
	// sentinels BEFORE checking the JSON envelope. The server emits these
	// as JSON envelopes with status=false, but callers usually want to
	// branch on the kind of failure rather than parse messages.
	if sentinel := classifyStatus(resp.StatusCode); sentinel != nil {
		return nil, fmt.Errorf("%w: %s", sentinel, statusReason(respBody, resp.StatusCode))
	}

	// Check for non-JSON responses (e.g. HTML error pages from proxies).
	if !json.Valid(respBody) {
		snippet := string(respBody)
		if len(snippet) > 256 {
			snippet = snippet[:256] + "..."
		}
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
			Message:    fmt.Sprintf("non-JSON response: %s", snippet),
		}
	}

	// Check the top-level status envelope.
	var envelope apiStatusEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("ibs: decode response envelope (HTTP %d): %w", resp.StatusCode, err)
	}

	if !envelope.Status {
		errMsg := envelope.Error
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
			Message:    errMsg,
		}
	}

	return respBody, nil
}
