package ibs

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestUpdateUserOwnership(t *testing.T) {
	wantBody := map[string]any{
		"user_id":     "user-123",
		"new_user_id": "new-user-456",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/card/update/ownership/user" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/card/update/ownership/user")
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
		_, _ = w.Write([]byte(`{"status":true,"data":{"updated_count":3}}`))
	}))
	t.Cleanup(server.Close)

	client := &Client{
		userID: "user-123",
		g: &globalConfig{
			apiURL:     server.URL,
			apiKey:     testAPIKey,
			secretKey:  testSecretKey,
			userAgent:  "IBS-SDK-Test/1.0",
			httpClient: server.Client(),
		},
	}

	updated, err := client.UpdateUserOwnership("new-user-456")
	if err != nil {
		t.Fatalf("UpdateUserOwnership failed: %v", err)
	}
	if updated != 3 {
		t.Fatalf("updated count = %d, want 3", updated)
	}
}
