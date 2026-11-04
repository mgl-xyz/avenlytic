package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookSend(t *testing.T) {
	var got Message
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method=%s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()
	err := (Webhook{}).Send(context.Background(), Config{Settings: map[string]string{"url": s.URL}}, Message{Title: "test", Text: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "test" {
		t.Fatalf("unexpected payload: %+v", got)
	}
}
func TestWebhookRejectsProviderError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"description":"bad token"}`))
	}))
	defer s.Close()
	err := (Webhook{}).Send(context.Background(), Config{Settings: map[string]string{"url": s.URL}}, Message{})
	if err == nil {
		t.Fatal("expected provider error")
	}
}
func TestRegistryRejectsUnknownChannel(t *testing.T) {
	err := NewRegistry().Send(context.Background(), Config{Type: "missing"}, Message{})
	if err == nil {
		t.Fatal("expected unsupported channel error")
	}
}
