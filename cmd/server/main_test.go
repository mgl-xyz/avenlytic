package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSPAFallbackRoutes(t *testing.T) {
	handler := spaFallback(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	for _, path := range []string{"/login", "/hosts", "/alerts", "/rules", "/channels", "/settings"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Body.String() != "/" {
			t.Errorf("route %s was served from %q, want /", path, response.Body.String())
		}
	}
}
