package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLINETargetAndRetryKey(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("X-Line-Retry-Key") != "57039d8b-c58e-4ee1-8943-539dc6203626" {
			t.Error("missing auth/retry key")
		}
		var body struct {
			To       string              `json:"to"`
			Messages []map[string]string `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.To != "specific-admin-user" || len(body.Messages) != 1 {
			t.Error("wrong LINE recipient")
		}
		if calls == 2 {
			w.Header().Set("X-Line-Accepted-Request-Id", "already-accepted")
			w.WriteHeader(409)
		}
	}))
	defer server.Close()
	sender := NewLINE(true, "test-token", "specific-admin-user")
	sender.endpoint = server.URL
	event := Event{ID: "57039d8b-c58e-4ee1-8943-539dc6203626", RequestID: "request", RequestNumber: 1}
	for range 2 {
		if err := sender.Send(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	sender.enabled = false
	if err := sender.Send(context.Background(), event); err != ErrDisabled {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("disabled LINE sent a request")
	}
}
