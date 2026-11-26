package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type tokenPayload struct {
	Acc string `json:"acc"`
	Exp int64  `json:"exp"`
}

type Auth struct {
	cfg      *Config
	secret   []byte
	mu       sync.RWMutex
	revoked  map[string]int64
	lifetime time.Duration
}

func newAuth(cfg *Config) *Auth {
	a := &Auth{cfg: cfg, revoked: map[string]int64{}, lifetime: time.Duration(cfg.SessionTimeout) * time.Minute}
	path := filepath.Join(cfg.Dir, ".modernfs_secret")
	if b, err := os.ReadFile(path); err == nil && len(b) >= 16 {
		a.secret = b
		return a
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err == nil {
		a.secret = b
		_ = os.WriteFile(path, b, 0600)
	}
	return a
}

func (a *Auth) sign(p tokenPayload) string {
	raw, _ := json.Marshal(p)
	body := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *Auth) verify(tok string) (tokenPayload, bool) {
	var p tokenPayload
	parts := strings.Split(tok, ".")
	if len(parts) != 2 {
		return p, false
	}
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(parts[0]))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(parts[1])) {
		return p, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(raw, &p) != nil {
		return p, false
	}
	if p.Exp < time.Now().Unix() {
		return p, false
	}
	a.mu.RLock()
	_, dead := a.revoked[tok]
	a.mu.RUnlock()
	if dead {
		return p, false
	}
	return p, true
}

func (a *Auth) issue(name string) (string, time.Time) {
	exp := time.Now().Add(a.lifetime)
	tok := a.sign(tokenPayload{Acc: name, Exp: exp.Unix()})
	return tok, exp
}

func (a *Auth) revoke(tok string) {
	a.mu.Lock()
	a.revoked[tok] = time.Now().Add(a.lifetime).Unix()
	a.mu.Unlock()
}

func (a *Auth) cookieToken(r *http.Request) string {
	c, err := r.Cookie("JWT")
	if err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}

func (a *Auth) Current(r *http.Request) (*User, string) {
	if tok := a.cookieToken(r); tok != "" {
		if p, ok := a.verify(tok); ok {
			if u := a.cfg.resolveUser(p.Acc); u != nil {
				return u, tok
			}
		}
	}
	if name, pwd, ok := r.BasicAuth(); ok {
		if u := a.cfg.checkPassword(name, pwd); u != nil {
			return u, ""
		}
	}
	return a.cfg.guest(), ""
}

func (a *Auth) setCookie(w http.ResponseWriter, tok string, exp time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "JWT",
		Value:    tok,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *Auth) clearCookie(w http.ResponseWriter, secure bool) {
	for _, name := range []string{"JWT", "user"} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure})
	}
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}
