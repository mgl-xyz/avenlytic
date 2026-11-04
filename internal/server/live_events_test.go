package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type streamTestWriter struct {
	header  http.Header
	mu      sync.Mutex
	body    bytes.Buffer
	flushed chan struct{}
}

func newStreamTestWriter() *streamTestWriter {
	return &streamTestWriter{header: make(http.Header), flushed: make(chan struct{}, 4)}
}

func (w *streamTestWriter) Header() http.Header { return w.header }

func (w *streamTestWriter) Write(body []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.Write(body)
}

func (w *streamTestWriter) WriteHeader(int) {}

func (w *streamTestWriter) Flush() {
	select {
	case w.flushed <- struct{}{}:
	default:
	}
}

func (w *streamTestWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String()
}

func TestLiveEventsRequireAdminAndPushUpdates(t *testing.T) {
	app := NewAt("secret", filepath.Join(t.TempDir(), "state.json"))
	unauthorized := httptest.NewRecorder()
	app.Handler().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/events", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized stream got %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	sessionWriter := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	app.createSession(sessionWriter, sessionRequest, "admin")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil).WithContext(ctx)
	request.AddCookie(sessionWriter.Result().Cookies()[0])
	writer := newStreamTestWriter()
	done := make(chan struct{})
	go func() {
		app.Handler().ServeHTTP(writer, request)
		close(done)
	}()

	select {
	case <-writer.flushed:
	case <-time.After(time.Second):
		t.Fatal("event stream did not establish")
	}
	if writer.header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("unexpected content type %q", writer.header.Get("Content-Type"))
	}

	app.publishUpdate()
	select {
	case <-writer.flushed:
	case <-time.After(time.Second):
		t.Fatal("update was not streamed")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("event stream did not close after cancellation")
	}
	body := writer.String()
	if strings.Count(body, "event: monitoring\ndata: ") != 2 {
		t.Fatalf("stream should send an initial snapshot and one update: %q", body)
	}
	if strings.Contains(body, "event: invalidate") {
		t.Fatalf("stream unexpectedly emitted an invalidation event: %q", body)
	}
}
