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
		{"slash becomes a dash", "a/b", "a-b", false},
		{"backslash becomes a dash", `a\b`, "a-b", false},
		{"space becomes a dash", "alice smith", "alice-smith", false},
		{"umlaut becomes a dash", "Müller", "m-ller", false},
		{"at sign becomes a dash", "alice@example.com", "alice-example.com", false},
		{"surrounding space is trimmed", "  alice  ", "alice", false},
		{"empty claim", "", "", true},
		{"only whitespace", "   ", "", true},
		{"current directory", ".", "", true},
		{"parent directory", "..", "", true},
		{"traversal is folded into dashes", "../../etc", "-.-..-etc", false},
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
