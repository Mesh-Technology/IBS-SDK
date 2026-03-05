package ibs

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const callbackMaxTimestampSkewSeconds = 60

var (
	ErrMissingCallbackHeaders   = errors.New("ibs: missing callback hmac headers")
	ErrInvalidCallbackTimestamp = errors.New("ibs: invalid callback timestamp")
	ErrStaleCallbackTimestamp   = errors.New("ibs: stale callback timestamp")
	ErrInvalidCallbackAPIKey    = errors.New("ibs: invalid callback api key")
	ErrInvalidCallbackSignature = errors.New("ibs: invalid callback signature")
	ErrInvalidCallbackPayload   = errors.New("ibs: invalid callback payload")
)

// ReverseCallbackEnvelope is the minimal envelope used to determine the event type.
type ReverseCallbackEnvelope struct {
	Event string `json:"event"`
}

// PendingTransactionReverseCallback represents a callback for a reversed pending transaction.
type PendingTransactionReverseCallback struct {
	Event              string `json:"event"`
	TransactionID      string `json:"transaction_id"`
	ServiceID          string `json:"service_id"`
	UserID             string `json:"user_id"`
	CardID             string `json:"card_id"`
	Amount             string `json:"amount"`
	NetAmount          string `json:"net_amount"`
	CommissionAmount   string `json:"commission_amount"`
	Currency           string `json:"currency"`
	ExternalRef        string `json:"external_ref"`
	Description        string `json:"description"`
	ExpectedRefund     string `json:"expected_refund"`
	ExpectedRefundType string `json:"expected_refund_type"`
	Reason             string `json:"reason"`
}

// PendingCardReverseCallback represents a callback for a reversed pending card activation.
type PendingCardReverseCallback struct {
	Event         string `json:"event"`
	OrderID       string `json:"order_id"`
	ServiceID     string `json:"service_id"`
	UserID        string `json:"user_id"`
	UserEmail     string `json:"user_email"`
	UserFullName  string `json:"user_full_name"`
	BankID        string `json:"bank_id"`
	CardType      string `json:"card_type"`
	PriceAmount   string `json:"price_amount"`
	PriceCurrency string `json:"price_currency"`
	Reason        string `json:"reason"`
}

// VerifyCallbackSignature verifies the HMAC-SHA512 signature of a callback payload
// using the provided API key, signature, timestamp, and raw body bytes.
func (c *Client) VerifyCallbackSignature(apiKey, signature, timestamp string, body []byte) error {
	receivedAPIKey := strings.TrimSpace(apiKey)
	receivedSignature := strings.TrimSpace(signature)
	receivedTimestamp := strings.TrimSpace(timestamp)

	if receivedAPIKey == "" || receivedSignature == "" || receivedTimestamp == "" {
		return ErrMissingCallbackHeaders
	}

	expectedAPIKey := strings.TrimSpace(c.g.apiKey)
	expectedSecretKey := strings.TrimSpace(c.g.secretKey)
	if expectedAPIKey == "" || expectedSecretKey == "" {
		return fmt.Errorf("ibs: callback verifier is not configured (missing api key or secret key)")
	}
	if receivedAPIKey != expectedAPIKey {
		return ErrInvalidCallbackAPIKey
	}

	ts, err := strconv.ParseInt(receivedTimestamp, 10, 64)
	if err != nil {
		return ErrInvalidCallbackTimestamp
	}

	now := time.Now().Unix()
	if ts < now-callbackMaxTimestampSkewSeconds || ts > now+callbackMaxTimestampSkewSeconds {
		return ErrStaleCallbackTimestamp
	}

	bodyForSign := make([]byte, len(body))
	copy(bodyForSign, body)
	expectedSignature, err := signCallbackBody(bodyForSign, expectedAPIKey, expectedSecretKey, receivedTimestamp)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(expectedSignature), []byte(receivedSignature)) {
		return ErrInvalidCallbackSignature
	}

	return nil
}

func signCallbackBody(body []byte, apiKey, secretKey, timestamp string) (string, error) {
	decodedSecretKey, err := base64.StdEncoding.DecodeString(secretKey)
	if err != nil {
		return "", fmt.Errorf("ibs: failed to decode secret key: %w", err)
	}

	message := make([]byte, 0, len(body)+len(apiKey)+len(timestamp))
	message = append(message, body...)
	message = append(message, []byte(apiKey)...)
	message = append(message, []byte(timestamp)...)

	mac := hmac.New(sha512.New, decodedSecretKey)
	mac.Write(message)

	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

// VerifyCallbackRequest verifies HMAC headers from an http.Request, reads the body,
// and resets the request body for subsequent reads. Returns the raw body bytes on success.
func (c *Client) VerifyCallbackRequest(r *http.Request) ([]byte, error) {
	if r == nil {
		return nil, errors.New("ibs: request is nil")
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("ibs: read callback body: %w", err)
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if err := c.VerifyCallbackSignature(
		r.Header.Get("X-Api-Key"),
		r.Header.Get("X-Signature"),
		r.Header.Get("X-Timestamp"),
		bodyBytes,
	); err != nil {
		return nil, err
	}

	return bodyBytes, nil
}

// ParseReverseCallbackEvent extracts the event name from a raw callback body.
func ParseReverseCallbackEvent(body []byte) (string, error) {
	var envelope ReverseCallbackEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("ibs: unmarshal callback envelope: %w", err)
	}

	event := strings.TrimSpace(envelope.Event)
	if event == "" {
		return "", ErrInvalidCallbackPayload
	}

	return event, nil
}

// ParsePendingTransactionReverseCallback parses a pending_transaction_reverse callback payload.
func ParsePendingTransactionReverseCallback(body []byte) (*PendingTransactionReverseCallback, error) {
	var callback PendingTransactionReverseCallback
	if err := json.Unmarshal(body, &callback); err != nil {
		return nil, fmt.Errorf("ibs: unmarshal pending transaction reverse callback: %w", err)
	}
	if strings.TrimSpace(callback.Event) != "pending_transaction_reverse" {
		return nil, ErrInvalidCallbackPayload
	}
	if strings.TrimSpace(callback.TransactionID) == "" {
		return nil, ErrInvalidCallbackPayload
	}
	return &callback, nil
}

// ParsePendingCardReverseCallback parses a pending_card_reverse callback payload.
func ParsePendingCardReverseCallback(body []byte) (*PendingCardReverseCallback, error) {
	var callback PendingCardReverseCallback
	if err := json.Unmarshal(body, &callback); err != nil {
		return nil, fmt.Errorf("ibs: unmarshal pending card reverse callback: %w", err)
	}
	if strings.TrimSpace(callback.Event) != "pending_card_reverse" {
		return nil, ErrInvalidCallbackPayload
	}
	if strings.TrimSpace(callback.OrderID) == "" {
		return nil, ErrInvalidCallbackPayload
	}
	return &callback, nil
}
