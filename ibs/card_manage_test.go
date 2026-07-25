package ibs

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

const managedCardJSON = `{
	"status": true,
	"data": {
		"card_id": "card-456",
		"provider_card_id": "provider-card-123",
		"bank_id": "papara",
		"user_id": "user-123",
		"user_full_name": "John Doe",
		"user_email": "user@example.com",
		"card_last4": "1111",
		"type": "physical",
		"enabled": true
	}
}`

func TestAddCard(t *testing.T) {
	wantBody := map[string]any{
		"bank_id":          "papara",
		"user_id":          "user-123",
		"user_full_name":   "John Doe",
		"user_email":       "user@example.com",
		"provider_card_id": "provider-card-123",
		"card_number":      "4111111111111111",
		"cvv":              "123",
		"expire_month":     "12",
		"expire_year":      "2029",
		"type":             "physical",
	}
	client := newManagedCardTestClient(t, "/card/add", wantBody)
	client.userID = "user-123"

	card, err := client.AddCard(ExistingCard{
		BankID:         "papara",
		UserFullName:   "John Doe",
		UserEmail:      "user@example.com",
		ProviderCardID: "provider-card-123",
		CardNumber:     "4111111111111111",
		Cvv:            "123",
		ExpireMonth:    "12",
		ExpireYear:     "2029",
		Type:           "physical",
	})
	if err != nil {
		t.Fatalf("AddCard failed: %v", err)
	}
	if card.CardID != "card-456" || card.CardLast4 != "1111" || !card.Enabled {
		t.Fatalf("unexpected card: %+v", card)
	}
}

func TestUpdateCardInfoOmitsEmptyFields(t *testing.T) {
	wantBody := map[string]any{
		"card_id":        "card-456",
		"user_id":        "user-123",
		"user_full_name": "John Doe",
	}
	client := newManagedCardTestClient(t, "/card/update/info", wantBody)
	client.cardID = "card-456"

	card, err := client.UpdateCardInfo(CardInfoUpdate{
		UserID:       "user-123",
		UserFullName: "John Doe",
	})
	if err != nil {
		t.Fatalf("UpdateCardInfo failed: %v", err)
	}
	if card.CardID != "card-456" {
		t.Fatalf("CardID = %q, want %q", card.CardID, "card-456")
	}
}

func TestDeleteCard(t *testing.T) {
	wantBody := map[string]any{
		"card_id": "card-456",
	}
	client := newManagedCardTestClient(t, "/card/delete/hard", wantBody)
	client.cardID = "card-456"

	if err := client.DeleteCard(); err != nil {
		t.Fatalf("DeleteCard failed: %v", err)
	}
}

func newManagedCardTestClient(t *testing.T, wantPath string, wantBody map[string]any) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var gotBody map[string]any
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if !reflect.DeepEqual(gotBody, wantBody) {
			t.Errorf("request body = %#v, want %#v", gotBody, wantBody)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(managedCardJSON))
	}))
	t.Cleanup(server.Close)

	return &Client{g: &globalConfig{
		apiURL:     server.URL,
		apiKey:     testAPIKey,
		secretKey:  testSecretKey,
		userAgent:  "IBS-SDK-Test/1.0",
		httpClient: server.Client(),
	}}
}
