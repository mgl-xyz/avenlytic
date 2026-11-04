package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/avenlytic/avenlytic/internal/storage"
)

const jwtIssuer = "avenlytic"
const jwtLifetime = 12 * time.Hour

type jwtClaims struct {
	Subject   string `json:"sub"`
	Issuer    string `json:"iss"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func (a *App) jwtSecret() (string, error) {
	var secret string
	a.store.View(func(state *storage.State) {
		secret = state.Auth.JWTSecret
	})
	if secret != "" {
		return secret, nil
	}
	generated, err := randomSecret(32)
	if err != nil {
		return "", err
	}
	err = a.store.Update(func(state *storage.State) {
		if state.Auth.JWTSecret == "" {
			state.Auth.JWTSecret = generated
		}
		secret = state.Auth.JWTSecret
	})
	if err != nil {
		return "", err
	}
	return secret, nil
}

func (a *App) issueJWTFor(username string) (string, time.Time, error) {
	secret, err := a.jwtSecret()
	if err != nil {
		return "", time.Time{}, err
	}
	now := time.Now().UTC()
	expires := now.Add(jwtLifetime)
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", time.Time{}, err
	}
	claims, err := json.Marshal(jwtClaims{Subject: username, Issuer: jwtIssuer, IssuedAt: now.Unix(), ExpiresAt: expires.Unix()})
	if err != nil {
		return "", time.Time{}, err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	token := unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return token, expires, nil
}

func (a *App) parseJWT(token string) (jwtClaims, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtClaims{}, false
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return jwtClaims{}, false
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Algorithm != "HS256" || header.Type != "JWT" {
		return jwtClaims{}, false
	}
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtClaims{}, false
	}
	var claims jwtClaims
	if json.Unmarshal(claimsBytes, &claims) != nil || claims.Issuer != jwtIssuer {
		return jwtClaims{}, false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return jwtClaims{}, false
	}
	var record storage.AuthRecord
	a.store.View(func(state *storage.State) { record = state.Auth })
	if record.Hash == "" || record.JWTSecret == "" || claims.Subject == "" || claims.Subject != record.Username {
		return jwtClaims{}, false
	}
	mac := hmac.New(sha256.New, []byte(record.JWTSecret))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := mac.Sum(nil)
	if len(signature) != len(expected) || subtle.ConstantTimeCompare(signature, expected) != 1 {
		return jwtClaims{}, false
	}
	now := time.Now().Unix()
	if claims.ExpiresAt <= now || claims.IssuedAt > now+30 || claims.IssuedAt > claims.ExpiresAt {
		return jwtClaims{}, false
	}
	return claims, true
}

func (a *App) bearerJWT(r *http.Request) (jwtClaims, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return jwtClaims{}, false
	}
	return a.parseJWT(strings.TrimPrefix(header, "Bearer "))
}

func (a *App) issueJWT(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	var record storage.AuthRecord
	a.store.View(func(state *storage.State) { record = state.Auth })
	if record.Hash == "" {
		http.Error(w, "initial setup required", http.StatusPreconditionRequired)
		return
	}
	salt, saltErr := hex.DecodeString(record.Salt)
	expected, hashErr := hex.DecodeString(record.Hash)
	if saltErr != nil || hashErr != nil || len(salt) != 16 || len(expected) != 32 {
		http.Error(w, "authentication unavailable", http.StatusInternalServerError)
		return
	}
	got := pbkdf2SHA256([]byte(in.Password), salt, passwordRounds, len(expected))
	if in.Username != record.Username || subtle.ConstantTimeCompare(got, expected) != 1 {
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}
	token, expires, err := a.issueJWTFor(record.Username)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_token": token, "token_type": "Bearer", "expires_at": expires})
}

func jwtExpires(claims jwtClaims) time.Time {
	return time.Unix(claims.ExpiresAt, 0).UTC()
}
