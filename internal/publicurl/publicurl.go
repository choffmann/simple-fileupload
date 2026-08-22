package publicurl

import (
	"fmt"
	"net/url"
	"strings"
)

// Path builds the host relative, percent escaped path of a user's file or
// directory. A subpath ending in a slash keeps it, so directory urls stay
// distinguishable from file urls.
func Path(username, subpath string) string {
	trimmed := strings.Trim(subpath, "/")

	segments := []string{username}
	if trimmed != "" {
		segments = append(segments, strings.Split(trimmed, "/")...)
	}

	escaped := make([]string, len(segments))
	for i, s := range segments {
		escaped[i] = url.PathEscape(s)
	}

	p := "/" + strings.Join(escaped, "/")
	if trimmed == "" || strings.HasSuffix(subpath, "/") {
		p += "/"
	}
	return p
}

// For prefixes Path with baseURL to form the absolute url a QR code points at.
func For(baseURL, username, subpath string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parsing base url %q: %w", baseURL, err)
	}
	return strings.TrimSuffix(u.String(), "/") + Path(username, subpath), nil
}
