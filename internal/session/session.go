package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const CookieName = "session"

type Manager struct {
	secret []byte
	ttl    time.Duration
	secure bool
}

func NewManager(secret []byte, ttl time.Duration, secure bool) *Manager {
	if len(secret) == 0 {
		panic("session: empty secret")
	}
	return &Manager{secret: secret, ttl: ttl, secure: secure}
}

func (m *Manager) Issue(w http.ResponseWriter, username string) {
	expiry := time.Now().Add(m.ttl)

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    m.sign(username, expiry),
		Path:     "/",
		Expires:  expiry,
		MaxAge:   int(m.ttl / time.Second),
		HttpOnly: true,
		Secure:   m.secure,
		// Lax is required: Strict would drop the cookie on the top-level
		// redirect back from the identity provider.
		SameSite: http.SameSiteLaxMode,
	})
}

// Username returns an empty string for anything that is not a currently valid,
// correctly signed cookie.
func (m *Manager) Username(r *http.Request) string {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}

	username, err := m.parse(cookie.Value)
	if err != nil {
		return ""
	}
	return username
}

func (m *Manager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *Manager) sign(username string, expiry time.Time) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(username)) +
		"." + strconv.FormatInt(expiry.Unix(), 10)
	return payload + "." + base64.RawURLEncoding.EncodeToString(m.mac(payload))
}

func (m *Manager) parse(value string) (string, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed cookie")
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("malformed signature")
	}
	if !hmac.Equal(signature, m.mac(parts[0]+"."+parts[1])) {
		return "", fmt.Errorf("bad signature")
	}

	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", fmt.Errorf("malformed expiry")
	}
	if time.Now().After(time.Unix(expiry, 0)) {
		return "", fmt.Errorf("expired")
	}

	username, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("malformed username")
	}
	if len(username) == 0 {
		return "", fmt.Errorf("empty username")
	}

	return string(username), nil
}

func (m *Manager) mac(payload string) []byte {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(payload))
	return h.Sum(nil)
}
