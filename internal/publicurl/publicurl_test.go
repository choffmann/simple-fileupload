package publicurl

import "testing"

func TestPath(t *testing.T) {
	tests := []struct {
		name     string
		username string
		subpath  string
		want     string
	}{
		{"area root", "alice", "", "/alice/"},
		{"plain file", "alice", "foo.pdf", "/alice/foo.pdf"},
		{"file with space", "alice", "mein bild.png", "/alice/mein%20bild.png"},
		{"file with umlaut", "alice", "Übung.pdf", "/alice/%C3%9Cbung.pdf"},
		{"nested file", "alice", "fotos/2026/img.png", "/alice/fotos/2026/img.png"},
		{"nested with spaces", "alice", "alte fotos/mein bild.png", "/alice/alte%20fotos/mein%20bild.png"},
		{"directory keeps slash", "alice", "fotos/", "/alice/fotos/"},
		{"nested directory keeps slash", "alice", "fotos/2026/", "/alice/fotos/2026/"},
		{"leading slash ignored", "alice", "/foo.pdf", "/alice/foo.pdf"},
		{"hash in name", "alice", "a#b.png", "/alice/a%23b.png"},
		{"question mark in name", "alice", "a?b.png", "/alice/a%3Fb.png"},
		{"percent in name", "alice", "Rabatt 20%.pdf", "/alice/Rabatt%2020%25.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Path(tt.username, tt.subpath); got != tt.want {
				t.Errorf("Path(%q, %q) = %q, want %q", tt.username, tt.subpath, got, tt.want)
			}
		})
	}
}

func TestFor(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		username string
		subpath  string
		want     string
	}{
		{"no trailing slash", "https://x.de", "alice", "foo.pdf", "https://x.de/alice/foo.pdf"},
		{"trailing slash", "https://x.de/", "alice", "foo.pdf", "https://x.de/alice/foo.pdf"},
		{"base with path", "https://x.de/files", "alice", "foo.pdf", "https://x.de/files/alice/foo.pdf"},
		{"localhost with port", "http://localhost:8080", "alice", "foo.pdf", "http://localhost:8080/alice/foo.pdf"},
		{"area root", "https://x.de", "alice", "", "https://x.de/alice/"},
		{"directory", "https://x.de", "alice", "fotos/", "https://x.de/alice/fotos/"},
		{"umlaut", "https://x.de", "alice", "Übung.pdf", "https://x.de/alice/%C3%9Cbung.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := For(tt.base, tt.username, tt.subpath)
			if err != nil {
				t.Fatalf("For(%q, %q, %q) returned error: %v", tt.base, tt.username, tt.subpath, err)
			}
			if got != tt.want {
				t.Errorf("For(%q, %q, %q) = %q, want %q", tt.base, tt.username, tt.subpath, got, tt.want)
			}
		})
	}
}

func TestForRejectsUnparsableBase(t *testing.T) {
	if _, err := For("http://[::1", "alice", ""); err == nil {
		t.Error("expected an error for an unparsable base url, got nil")
	}
}
