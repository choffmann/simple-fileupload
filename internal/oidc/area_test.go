package oidc

import "testing"

func TestAreaName(t *testing.T) {
	tests := []struct {
		name    string
		claim   string
		want    string
		wantErr bool
	}{
		{"plain username", "alice", "alice", false},
		{"uppercase is folded", "Alice", "alice", false},
		{"dots and dashes survive", "alice.b_c-d", "alice.b_c-d", false},
		{"digits survive", "user42", "user42", false},
		{"surrounding space is trimmed", "  alice  ", "alice", false},
		{"slash is rejected", "a/b", "", true},
		{"backslash is rejected", `a\b`, "", true},
		{"inner space is rejected", "alice smith", "", true},
		{"umlaut is rejected", "Müller", "", true},
		{"email is rejected", "alice@example.com", "", true},
		{"traversal is rejected", "../../etc", "", true},
		{"empty claim", "", "", true},
		{"only whitespace", "   ", "", true},
		{"current directory", ".", "", true},
		{"parent directory", "..", "", true},
		{"reserved auth", "auth", "", true},
		{"reserved qr", "QR", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AreaName(tt.claim)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("AreaName(%q) returned %q, want an error", tt.claim, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("AreaName(%q) returned an unexpected error: %v", tt.claim, err)
			}
			if got != tt.want {
				t.Errorf("AreaName(%q) = %q, want %q", tt.claim, got, tt.want)
			}
		})
	}
}

func TestAreaNameNeverCollides(t *testing.T) {
	// pairs the old folding conversion mapped onto one shared area
	pairs := [][2]string{
		{"a/b", "a-b"},
		{`a\b`, "a-b"},
		{"a b", "a-b"},
		{"a:b", "a-b"},
		{"a+b", "a-b"},
		{"alice@example.com", "alice-example.com"},
		{"Müller", "m-ller"},
	}
	for _, pair := range pairs {
		first, firstErr := AreaName(pair[0])
		second, secondErr := AreaName(pair[1])

		if firstErr == nil && secondErr == nil && first == second {
			t.Errorf("AreaName(%q) and AreaName(%q) both yield %q", pair[0], pair[1], first)
		}
	}
}
