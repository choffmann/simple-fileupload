package config

import (
	"crypto/rand"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// RequireBaseURL returns the public base url without a trailing slash. QR codes
// are useless without an absolute url, so a missing or malformed value is a
// startup error rather than something to guess from request headers.
func RequireBaseURL() (string, error) {
	raw := strings.TrimSpace(os.Getenv("BASE_URL"))
	if raw == "" {
		return "", fmt.Errorf("BASE_URL is not set")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parsing BASE_URL %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("BASE_URL %q needs an http or https scheme", raw)
	}
	if u.Host == "" {
		return "", fmt.Errorf("BASE_URL %q has no host", raw)
	}

	return strings.TrimSuffix(raw, "/"), nil
}

func UploadDir() string {
	v := strings.TrimSpace(os.Getenv("UPLOAD_DIR"))
	if v == "" {
		return "./data"
	}
	return v
}

type OIDC struct {
	Issuer       string
	ClientID     string
	ClientSecret string
}

// RequireOIDC reads the identity provider settings. Everything is mandatory:
// without a provider nobody can upload anything, so guessing is pointless.
func RequireOIDC() (OIDC, error) {
	issuer := strings.TrimSuffix(strings.TrimSpace(os.Getenv("OIDC_ISSUER")), "/")
	if issuer == "" {
		return OIDC{}, fmt.Errorf("OIDC_ISSUER is not set")
	}

	u, err := url.Parse(issuer)
	if err != nil {
		return OIDC{}, fmt.Errorf("parsing OIDC_ISSUER %q: %w", issuer, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return OIDC{}, fmt.Errorf("OIDC_ISSUER %q needs an http or https scheme", issuer)
	}
	if u.Host == "" {
		return OIDC{}, fmt.Errorf("OIDC_ISSUER %q has no host", issuer)
	}

	clientID := strings.TrimSpace(os.Getenv("OIDC_CLIENT_ID"))
	if clientID == "" {
		return OIDC{}, fmt.Errorf("OIDC_CLIENT_ID is not set")
	}

	clientSecret := strings.TrimSpace(os.Getenv("OIDC_CLIENT_SECRET"))
	if clientSecret == "" {
		return OIDC{}, fmt.Errorf("OIDC_CLIENT_SECRET is not set")
	}

	return OIDC{Issuer: issuer, ClientID: clientID, ClientSecret: clientSecret}, nil
}

// SessionSecret returns the cookie signing key. The bool reports that a random
// key was generated because SESSION_SECRET was unset, which means every restart
// invalidates all sessions.
func SessionSecret() ([]byte, bool, error) {
	if v := strings.TrimSpace(os.Getenv("SESSION_SECRET")); v != "" {
		if len(v) < 16 {
			return nil, false, fmt.Errorf("SESSION_SECRET needs at least 16 characters")
		}
		return []byte(v), false, nil
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, false, fmt.Errorf("generating a session secret: %w", err)
	}
	return secret, true, nil
}

// Origin returns the scheme and host of a validated base url. That is what a
// browser puts in the Origin header; BASE_URL may carry a path, an Origin never does.
func Origin(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parsing %q: %w", baseURL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("%q has no scheme or host", baseURL)
	}

	return u.Scheme + "://" + u.Host, nil
}
