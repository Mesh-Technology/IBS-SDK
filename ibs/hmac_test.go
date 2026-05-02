package ibs

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

const (
	testAPIKey    = "test-api-key"
	testSecretKey = "c2VjcmV0LWtleQ==" // base64("secret-key")
)

func TestRequestAPISendsNonceAndHexSignature(t *testing.T) {
	client := &Client{g: &globalConfig{
		apiKey:     testAPIKey,
		secretKey:  testSecretKey,
		userAgent:  "IBS-SDK-Test/1.0",
		httpClient: http.DefaultClient,
	}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		if got := r.Header.Get("X-Api-Key"); got != testAPIKey {
			t.Fatalf("X-Api-Key = %q, want %q", got, testAPIKey)
		}
		timestamp := r.Header.Get("X-Timestamp")
		if timestamp == "" {
			t.Fatal("missing X-Timestamp")
		}
		nonce := r.Header.Get("X-Nonce")
		if nonce == "" {
			t.Fatal("missing X-Nonce")
		}
		if _, err := hex.DecodeString(nonce); err != nil {
			t.Fatalf("nonce is not hex: %v", err)
		}

		signature := r.Header.Get("X-Signature")
		if _, err := hex.DecodeString(signature); err != nil {
			t.Fatalf("signature is not hex: %v", err)
		}
		expectedSignature, err := client.sign(body, timestamp, nonce)
		if err != nil {
			t.Fatalf("sign request body: %v", err)
		}
		if signature != expectedSignature {
			t.Fatalf("signature mismatch: got %q want %q", signature, expectedSignature)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true}`))
	}))
	t.Cleanup(server.Close)
	client.g.apiURL = server.URL

	if _, err := client.requestAPI(http.MethodPost, "/v1/card/balance/add", map[string]any{"amount": 10}, true); err != nil {
		t.Fatalf("requestAPI failed: %v", err)
	}
}

func TestVerifyCallbackSignatureRequiresNonceAndHexSignature(t *testing.T) {
	client := &Client{g: &globalConfig{
		apiKey:    testAPIKey,
		secretKey: testSecretKey,
	}}

	body := []byte(`{"event":"pending_transaction_reverse","transaction_id":"tx-1"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "00112233445566778899aabbccddeeff"
	signature, err := signCallbackBody(body, testAPIKey, testSecretKey, timestamp, nonce)
	if err != nil {
		t.Fatalf("sign callback body: %v", err)
	}
	if _, err := hex.DecodeString(signature); err != nil {
		t.Fatalf("callback signature is not hex: %v", err)
	}

	if err := client.VerifyCallbackSignature(testAPIKey, signature, timestamp, nonce, body); err != nil {
		t.Fatalf("valid callback signature rejected: %v", err)
	}
	if err := client.VerifyCallbackSignature(testAPIKey, signature, timestamp, "", body); !errors.Is(err, ErrMissingCallbackHeaders) {
		t.Fatalf("missing nonce error = %v, want %v", err, ErrMissingCallbackHeaders)
	}
	if err := client.VerifyCallbackSignature(testAPIKey, signature, timestamp, "different-nonce", body); !errors.Is(err, ErrInvalidCallbackSignature) {
		t.Fatalf("changed nonce error = %v, want %v", err, ErrInvalidCallbackSignature)
	}
}

func TestVerifyCallbackRequestReadsNonceHeader(t *testing.T) {
	client := &Client{g: &globalConfig{
		apiKey:    testAPIKey,
		secretKey: testSecretKey,
	}}

	body := []byte(`{"event":"pending_card_reverse","order_id":"order-1"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "ffeeddccbbaa99887766554433221100"
	signature, err := signCallbackBody(body, testAPIKey, testSecretKey, timestamp, nonce)
	if err != nil {
		t.Fatalf("sign callback body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader(body))
	req.Header.Set("X-Api-Key", testAPIKey)
	req.Header.Set("X-Signature", signature)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)

	got, err := client.VerifyCallbackRequest(req)
	if err != nil {
		t.Fatalf("VerifyCallbackRequest failed: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("body mismatch: got %q want %q", got, body)
	}
}
