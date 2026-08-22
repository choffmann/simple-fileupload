package oidc

import (
	"fmt"
	"strings"
)

// auth and qr shadow the /auth/... and /qr/{username}/{path...} routes, so an
// area with either name would be partly unreachable.
var reservedAreaNames = map[string]bool{"auth": true, "qr": true}

// AreaName turns the preferred_username claim into a directory and url segment.
// A claim outside the allowed set is rejected rather than folded, because folding
// mapped distinct users onto one shared area. Case is the one fold left, and
// Keycloak already normalises usernames to lowercase.
func AreaName(claim string) (string, error) {
	claim = strings.TrimSpace(claim)
	if claim == "" {
		return "", fmt.Errorf("preferred_username is missing or empty")
	}

	name := strings.ToLower(claim)
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
		default:
			return "", fmt.Errorf("preferred_username %q contains %q, which cannot be part of an area name", claim, r)
		}
	}

	if name == "." || name == ".." || reservedAreaNames[name] {
		return "", fmt.Errorf("preferred_username %q is not a usable area name", claim)
	}

	return name, nil
}
