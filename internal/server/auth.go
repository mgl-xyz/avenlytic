package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/avenlytic/avenlytic/internal/storage"
)

const sessionCookie = "avenlytic_session"
const passwordRounds = 310000
const minPasswordLength = 6

var errPasswordTooShort = errors.New("password must contain at least 6 characters")

type authRecord struct {
	Username  string    `json:"username"`
	Salt      string    `json:"salt"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
}
type sessionRecord struct {
	Username string
	Expires  time.Time
}

func randomSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func pbkdf2SHA256(password, salt []byte, rounds, keyLen int) []byte {
	hLen := sha256.Size
	blocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, blocks*hLen)
	for i := 1; i <= blocks; i++ {
		msg := make([]byte, len(salt)+4)
		copy(msg, salt)
		msg[len(salt)] = byte(i >> 24)
		msg[len(salt)+1] = byte(i >> 16)
		msg[len(salt)+2] = byte(i >> 8)
		msg[len(salt)+3] = byte(i)
		mac := hmac.New(sha256.New, password)
		mac.Write(msg)
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for j := 1; j < rounds; j++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for k := range t {
				t[k] ^= u[k]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}
func hashPassword(password string) (salt, hash string, err error) {
	if utf8.RuneCountInString(password) < minPasswordLength {
		return "", "", errPasswordTooShort
	}
	s := make([]byte, 16)
	if _, err = rand.Read(s); err != nil {
		return
	}
	h := pbkdf2SHA256([]byte(password), s, passwordRounds, 32)
	return hex.EncodeToString(s), hex.EncodeToString(h), nil
}
func (a *App) authStatus(w http.ResponseWriter, r *http.Request) {
	initialized := false
	a.store.View(func(s *storage.State) { initialized = s.Auth.Hash != "" })
	writeJSON(w, 200, map[string]bool{"initialized": initialized})
}
func (a *App) setupAdmin(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password string }
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(strings.TrimSpace(in.Username)) < 3 || len(in.Username) > 64 {
		http.Error(w, "username must be 3-64 characters", 400)
		return
	}
	salt, hash, e := hashPassword(in.Password)
	if e != nil {
		http.Error(w, errPasswordTooShort.Error(), 400)
		return
	}
	jwtSecret, e := randomSecret(32)
	if e != nil {
		http.Error(w, "failed to initialize token signing key", http.StatusInternalServerError)
		return
	}
	created := false
	e = a.store.Update(func(s *storage.State) {
		if s.Auth.Hash == "" {
			s.Auth = storage.AuthRecord{Username: in.Username, Salt: salt, Hash: hash, JWTSecret: jwtSecret, CreatedAt: time.Now().UTC()}
			created = true
		}
	})
	if e != nil {
		http.Error(w, "failed to save administrator", 500)
		return
	}
	if !created {
		http.Error(w, "initial setup already completed", 409)
		return
	}
	a.createSession(w, r, in.Username)
	writeJSON(w, 201, map[string]any{"username": in.Username})
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password string }
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	var rec storage.AuthRecord
	a.store.View(func(s *storage.State) { rec = s.Auth })
	if rec.Hash == "" {
		http.Error(w, "initial setup required", 428)
		return
	}
	salt, e1 := hex.DecodeString(rec.Salt)
	expected, e2 := hex.DecodeString(rec.Hash)
	if e1 != nil || e2 != nil || len(salt) != 16 || len(expected) != 32 {
		http.Error(w, "authentication unavailable", 500)
		return
	}
	got := pbkdf2SHA256([]byte(in.Password), salt, passwordRounds, len(expected))
	if in.Username != rec.Username || subtle.ConstantTimeCompare(got, expected) != 1 {
		http.Error(w, "invalid username or password", 401)
		return
	}
	a.createSession(w, r, in.Username)
	writeJSON(w, 200, map[string]any{"username": in.Username})
}
func (a *App) createSession(w http.ResponseWriter, r *http.Request, username string) {
	id, _ := randomSecret(32)
	a.authMu.Lock()
	a.sessions[id] = sessionRecord{Username: username, Expires: time.Now().Add(12 * time.Hour)}
	a.authMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: id, Path: "/", HttpOnly: true, Secure: r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"), SameSite: http.SameSiteStrictMode, Expires: time.Now().Add(12 * time.Hour), MaxAge: 43200})
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie(sessionCookie); e == nil {
		a.authMu.Lock()
		delete(a.sessions, c.Value)
		a.authMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"), SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(204)
}
func (a *App) me(w http.ResponseWriter, r *http.Request) {
	if !a.hasSession(r) {
		if claims, ok := a.bearerJWT(r); ok {
			writeJSON(w, 200, map[string]any{"username": claims.Subject, "expires_at": jwtExpires(claims)})
			return
		} else {
			http.Error(w, "unauthorized", 401)
			return
		}
	}
	c, _ := r.Cookie(sessionCookie)
	a.authMu.Lock()
	s := a.sessions[cookieValue(c)]
	a.authMu.Unlock()
	writeJSON(w, 200, map[string]any{"username": s.Username, "expires_at": s.Expires})
}
func cookieValue(c *http.Cookie) string {
	if c == nil {
		return ""
	}
	return c.Value
}
func (a *App) changePassword(w http.ResponseWriter, r *http.Request) {
	if !a.hasSession(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	var in struct{ Current, Password string }
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	var rec storage.AuthRecord
	a.store.View(func(s *storage.State) { rec = s.Auth })
	salt, _ := hex.DecodeString(rec.Salt)
	expected, _ := hex.DecodeString(rec.Hash)
	got := pbkdf2SHA256([]byte(in.Current), salt, passwordRounds, len(expected))
	if subtle.ConstantTimeCompare(got, expected) != 1 {
		http.Error(w, "current password is incorrect", 403)
		return
	}
	newSalt, newHash, e := hashPassword(in.Password)
	if e != nil {
		http.Error(w, errPasswordTooShort.Error(), 400)
		return
	}
	jwtSecret, e := randomSecret(32)
	if e != nil {
		http.Error(w, "failed to rotate token signing key", http.StatusInternalServerError)
		return
	}
	if e = a.store.Update(func(s *storage.State) {
		s.Auth.Salt = newSalt
		s.Auth.Hash = newHash
		s.Auth.JWTSecret = jwtSecret
	}); e != nil {
		http.Error(w, "save failed", 500)
		return
	}
	a.authMu.Lock()
	a.sessions = map[string]sessionRecord{}
	a.authMu.Unlock()
	a.createSession(w, r, rec.Username)
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (a *App) hasSession(r *http.Request) bool {
	c, e := r.Cookie(sessionCookie)
	if e != nil {
		return false
	}
	a.authMu.Lock()
	defer a.authMu.Unlock()
	s, ok := a.sessions[c.Value]
	if !ok {
		return false
	}
	if time.Now().After(s.Expires) {
		delete(a.sessions, c.Value)
		return false
	}
	return true
}
