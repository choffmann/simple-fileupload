package session

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func requestWithSession(t *testing.T, m *Manager, username string) *http.Request {
	t.Helper()

	rec := httptest.NewRecorder()
	m.Issue(rec, username)

	req := httptest.NewRequest("GET", "/", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func TestIssuedCookieRoundTrips(t *testing.T) {
	m := NewManager([]byte("a-secret-of-decent-length"), time.Hour, false)

	rec := httptest.NewRecorder()
	m.Issue(rec, "alice")
	cookie := rec.Result().Cookies()[0]

	if !cookie.HttpOnly {
		t.Error("got HttpOnly false, want true")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("got SameSite %v, want %v", cookie.SameSite, http.SameSiteLaxMode)
	}
	if cookie.Path != "/" {
		t.Errorf("got Path %q, want %q", cookie.Path, "/")
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookie)

	if got := m.Username(req); got != "alice" {
		t.Errorf("got username %q, want %q", got, "alice")
	}
}

func TestUsernameWithNonASCIIRoundTrips(t *testing.T) {
	m := NewManager([]byte("a-secret-of-decent-length"), time.Hour, false)

	req := requestWithSession(t, m, "jörg")

	if got := m.Username(req); got != "jörg" {
		t.Errorf("got username %q, want %q", got, "jörg")
	}
}

func TestExpiredCookieIsRejected(t *testing.T) {
	m := NewManager([]byte("a-secret-of-decent-length"), time.Hour, false)

	value := m.sign("alice", time.Now().Add(-time.Minute))
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: value})

	if got := m.Username(req); got != "" {
		t.Errorf("got username %q, want it rejected", got)
	}
}

func TestCookieSignedWithAnotherSecretIsRejected(t *testing.T) {
	issuer := NewManager([]byte("a-secret-of-decent-length"), time.Hour, false)
	reader := NewManager([]byte("a-different-secret-value!"), time.Hour, false)

	req := requestWithSession(t, issuer, "alice")

	if got := reader.Username(req); got != "" {
		t.Errorf("got username %q, want it rejected", got)
	}
}

func TestTamperedCookieIsRejected(t *testing.T) {
	m := NewManager([]byte("a-secret-of-decent-length"), time.Hour, false)

	rec := httptest.NewRecorder()
	m.Issue(rec, "alice")
	cookie := rec.Result().Cookies()[0]

	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 3 {
		t.Fatalf("got %d cookie parts, want 3 (value: %q)", len(parts), cookie.Value)
	}
	// swap the username half for "mallory" while keeping the original signature
	mallory := base64.RawURLEncoding.EncodeToString([]byte("mallory"))
	cookie.Value = mallory + "." + parts[1] + "." + parts[2]

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookie)

	if got := m.Username(req); got != "" {
		t.Errorf("got username %q, want it rejected", got)
	}
}

func TestTamperedExpiryIsRejected(t *testing.T) {
	m := NewManager([]byte("a-secret-of-decent-length"), time.Hour, false)

	rec := httptest.NewRecorder()
	m.Issue(rec, "alice")
	cookie := rec.Result().Cookies()[0]

	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 3 {
		t.Fatalf("got %d cookie parts, want 3 (value: %q)", len(parts), cookie.Value)
	}
	// swap the expiry half for a far-future timestamp while keeping the original signature
	farFuture := strconv.FormatInt(time.Now().AddDate(100, 0, 0).Unix(), 10)
	cookie.Value = parts[0] + "." + farFuture + "." + parts[2]

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookie)

	if got := m.Username(req); got != "" {
		t.Errorf("got username %q, want it rejected", got)
	}
}

func TestGarbageCookieIsRejected(t *testing.T) {
	m := NewManager([]byte("a-secret-of-decent-length"), time.Hour, false)

	for _, value := range []string{"", "nonsense", "a.b", "a.b.c.d", "a.notanumber.c"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: CookieName, Value: value})

		if got := m.Username(req); got != "" {
			t.Errorf("value %q: got username %q, want it rejected", value, got)
		}
	}
}

func TestMissingCookieYieldsEmptyUsername(t *testing.T) {
	m := NewManager([]byte("a-secret-of-decent-length"), time.Hour, false)

	if got := m.Username(httptest.NewRequest("GET", "/", nil)); got != "" {
		t.Errorf("got username %q, want an empty string", got)
	}
}

func TestClearExpiresTheCookie(t *testing.T) {
	m := NewManager([]byte("a-secret-of-decent-length"), time.Hour, false)

	rec := httptest.NewRecorder()
	m.Clear(rec)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	if cookies[0].Name != CookieName {
		t.Errorf("got cookie name %q, want %q", cookies[0].Name, CookieName)
	}
	if cookies[0].Path != "/" {
		t.Errorf("got Path %q, want %q", cookies[0].Path, "/")
	}
	if cookies[0].MaxAge >= 0 {
		t.Errorf("got MaxAge %d, want a negative value", cookies[0].MaxAge)
	}
}

func TestSecureFlagFollowsConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		secure bool
	}{
		{"https deployment", true},
		{"plain http deployment", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager([]byte("a-secret-of-decent-length"), time.Hour, tt.secure)

			rec := httptest.NewRecorder()
			m.Issue(rec, "alice")

			if got := rec.Result().Cookies()[0].Secure; got != tt.secure {
				t.Errorf("got Secure %v, want %v", got, tt.secure)
			}
		})
	}
}

func TestEmptyUsernameIsRejected(t *testing.T) {
	m := NewManager([]byte("a-secret-of-decent-length"), time.Hour, false)

	value := m.sign("", time.Now().Add(time.Hour))

	if _, err := m.parse(value); err == nil {
		t.Error("got nil error for an empty username, want it rejected")
	}
}

func TestNewManagerPanicsOnEmptySecret(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected NewManager to panic on an empty secret")
		}
	}()
	NewManager(nil, time.Hour, false)
}
