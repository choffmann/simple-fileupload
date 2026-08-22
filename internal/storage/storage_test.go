package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveFileReturnsStoredName(t *testing.T) {
	tests := []struct {
		name     string
		upload   string
		wantName string
	}{
		{"plain name", "foo.pdf", "foo.pdf"},
		{"strips directory", "../evil.txt", "evil.txt"},
		{"keeps umlaut", "Übung 1.pdf", "Übung 1.pdf"},
		{"empty becomes unnamed", "", "unnamed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(t.TempDir())
			got, err := s.SaveFile("alice", "", strings.NewReader("hello"), tt.upload)
			if err != nil {
				t.Fatalf("SaveFile returned error: %v", err)
			}
			if got != tt.wantName {
				t.Fatalf("SaveFile returned name %q, want %q", got, tt.wantName)
			}
			data, err := os.ReadFile(filepath.Join(s.BaseDir, "alice", got))
			if err != nil {
				t.Fatalf("reading back the saved file: %v", err)
			}
			if string(data) != "hello" {
				t.Errorf("stored content is %q, want %q", data, "hello")
			}
		})
	}
}

func TestSaveFileCreatesSubdirectory(t *testing.T) {
	s := New(t.TempDir())
	got, err := s.SaveFile("alice", "alte fotos", strings.NewReader("x"), "bild.png")
	if err != nil {
		t.Fatalf("SaveFile returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.BaseDir, "alice", "alte fotos", got)); err != nil {
		t.Errorf("expected file inside the subdirectory: %v", err)
	}
}

func TestResolvePathStaysInsideUserDir(t *testing.T) {
	s := New(t.TempDir())
	userDir := filepath.Join(s.BaseDir, "alice")

	for _, subpath := range []string{"../../etc/passwd", "..", "a/../../b", "/etc/passwd", "./../bob"} {
		got, err := s.ResolvePath("alice", subpath)
		if err != nil {
			continue
		}
		if got != userDir && !strings.HasPrefix(got, userDir+string(filepath.Separator)) {
			t.Errorf("subpath %q resolved to %q, which escapes %q", subpath, got, userDir)
		}
	}
}

func TestUsernameCannotEscapeBaseDir(t *testing.T) {
	s := New(t.TempDir())

	for _, username := range []string{"../../etc", "..", ".", "", "a/b", `a\b`, "/etc", "alice/../bob"} {
		if got, err := s.ResolvePath(username, ""); err == nil {
			t.Errorf("ResolvePath(%q, \"\") returned %q, want an error", username, got)
		}
		if s.UserExists(username) {
			t.Errorf("UserExists(%q) is true, want false", username)
		}
		if err := s.EnsureUserDir(username); err == nil {
			t.Errorf("EnsureUserDir(%q) succeeded, want an error", username)
		}
	}
}
