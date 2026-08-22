package config

import (
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

func UsersFile() string {
	return strings.TrimSpace(os.Getenv("USERS_FILE"))
}
