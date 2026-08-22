package config

import "testing"

func TestRequireBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		want    string
		wantErr bool
	}{
		{"https", "https://files.example.com", "https://files.example.com", false},
		{"trailing slash trimmed", "https://files.example.com/", "https://files.example.com", false},
		{"localhost with port", "http://localhost:8080", "http://localhost:8080", false},
		{"surrounding space trimmed", "  https://files.example.com  ", "https://files.example.com", false},
		{"subpath kept", "https://files.example.com/share", "https://files.example.com/share", false},
		{"empty", "", "", true},
		{"missing scheme", "files.example.com", "", true},
		{"missing host", "https://", "", true},
		{"unsupported scheme", "ftp://files.example.com", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BASE_URL", tt.env)
			got, err := RequireBaseURL()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("RequireBaseURL() with %q returned %q, want an error", tt.env, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("RequireBaseURL() with %q returned error: %v", tt.env, err)
			}
			if got != tt.want {
				t.Errorf("RequireBaseURL() with %q = %q, want %q", tt.env, got, tt.want)
			}
		})
	}
}
