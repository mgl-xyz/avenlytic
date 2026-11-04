package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/avenlytic/avenlytic/internal/storage"
)

const uiGateCookie = "avenlytic_ui_gate"
const uiGateSessionDuration = 12 * time.Hour

func validUIGatePath(value string) bool {
	if len(value) < 3 || len(value) > 128 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func hashUIGatePath(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (a *App) currentUIGateHash() string {
	var hash string
	a.store.View(func(state *storage.State) {
		hash = state.Settings.UIGateHash
	})
	return hash
}

func (a *App) uiGateAuthorized(r *http.Request) bool {
	if a.currentUIGateHash() == "" {
		return true
	}
	cookie, err := r.Cookie(uiGateCookie)
	if err != nil {
		return false
	}
	a.gateMu.Lock()
	defer a.gateMu.Unlock()
	expires, ok := a.gateSessions[cookie.Value]
	if !ok {
		return false
	}
	if time.Now().After(expires) {
		delete(a.gateSessions, cookie.Value)
		return false
	}
	return true
}

func (a *App) createUIGateSession(w http.ResponseWriter, r *http.Request) error {
	id, err := randomSecret(32)
	if err != nil {
		return err
	}
	a.gateMu.Lock()
	now := time.Now()
	for session, expires := range a.gateSessions {
		if now.After(expires) {
			delete(a.gateSessions, session)
		}
	}
	a.gateSessions[id] = now.Add(uiGateSessionDuration)
	a.gateMu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     uiGateCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(uiGateSessionDuration),
		MaxAge:   int(uiGateSessionDuration.Seconds()),
	})
	return nil
}

func clearUIGateCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     uiGateCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func (a *App) enterUIGate(w http.ResponseWriter, r *http.Request, path string) {
	hash := a.currentUIGateHash()
	if hash == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if a.uiGateAuthorized(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	candidate := strings.TrimPrefix(path, "/")
	if r.Method != http.MethodGet || !strings.HasPrefix(path, "/") || strings.Contains(candidate, "/") || subtle.ConstantTimeCompare([]byte(hash), []byte(hashUIGatePath(candidate))) != 1 {
		http.NotFound(w, r)
		return
	}
	if err := a.createUIGateSession(w, r); err != nil {
		http.Error(w, "failed to create access session", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) uiGateEntry(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	a.enterUIGate(w, r, r.URL.Query().Get("path"))
}

func (a *App) UIHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.currentUIGateHash() == "" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		path := r.URL.Path
		candidate := strings.TrimPrefix(path, "/")
		if r.Method == http.MethodGet && strings.HasPrefix(path, "/") && !strings.Contains(candidate, "/") {
			hash := hashUIGatePath(candidate)
			if subtle.ConstantTimeCompare([]byte(a.currentUIGateHash()), []byte(hash)) == 1 {
				a.enterUIGate(w, r, path)
				return
			}
		}
		if !a.uiGateAuthorized(r) {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
