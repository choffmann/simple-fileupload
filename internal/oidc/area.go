package oidc

import (
	"fmt"
	"strings"
)

// auth and qr shadow the /auth/... and /qr/{username}/{path...} routes, so an
// area with either name would be partly unreachable.
var reservedAreaNames = map[string]bool{"auth": true, "qr": true}

// AreaName turns the preferred_username claim into a directory and url segment.
// Anything outside the allowed set becomes a dash rather than being dropped, so
// two different claims stay two different areas.
func AreaName(claim string) (string, error) {
	claim = strings.TrimSpace(claim)
	if claim == "" {
		return "", fmt.Errorf("preferred_username is missing or empty")
	}

	var b strings.Builder
	claim = strings.ToLower(claim)
	for i, r := range claim {
		switch {
		case i == 0 && r == '.':
			b.WriteRune('-')
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}

	name := b.String()
	if strings.TrimFunc(name, func(r rune) bool { return r == '.' || r == '-' }) == "" || reservedAreaNames[name] {
		return "", fmt.Errorf("preferred_username %q is not a usable area name", claim)
	}

	return name, nil
}
