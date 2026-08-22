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

func TestRequireOIDC(t *testing.T) {
	tests := []struct {
		name         string
		issuer       string
		clientID     string
		clientSecret string
		wantIssuer   string
		wantErr      bool
	}{
		{"complete", "https://kc.example.com/realms/x", "app", "s3cret", "https://kc.example.com/realms/x", false},
		{"trailing slash trimmed", "https://kc.example.com/realms/x/", "app", "s3cret", "https://kc.example.com/realms/x", false},
		{"surrounding space trimmed", "  https://kc.example.com/realms/x  ", "app", "s3cret", "https://kc.example.com/realms/x", false},
		{"http allowed for local setups", "http://localhost:8081/realms/x", "app", "s3cret", "http://localhost:8081/realms/x", false},
		{"issuer missing", "", "app", "s3cret", "", true},
		{"issuer without scheme", "kc.example.com/realms/x", "app", "s3cret", "", true},
		{"issuer without host", "https://", "app", "s3cret", "", true},
		{"issuer with unsupported scheme", "ldap://kc.example.com", "app", "s3cret", "", true},
		{"client id missing", "https://kc.example.com/realms/x", "", "s3cret", "", true},
		{"client secret missing", "https://kc.example.com/realms/x", "app", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OIDC_ISSUER", tt.issuer)
			t.Setenv("OIDC_CLIENT_ID", tt.clientID)
			t.Setenv("OIDC_CLIENT_SECRET", tt.clientSecret)

			got, err := RequireOIDC()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("RequireOIDC() returned %+v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("RequireOIDC() returned an unexpected error: %v", err)
			}
			if got.Issuer != tt.wantIssuer {
				t.Errorf("got Issuer %q, want %q", got.Issuer, tt.wantIssuer)
			}
			if got.ClientID != tt.clientID {
				t.Errorf("got ClientID %q, want %q", got.ClientID, tt.clientID)
			}
			if got.ClientSecret != tt.clientSecret {
				t.Errorf("got ClientSecret %q, want %q", got.ClientSecret, tt.clientSecret)
			}
		})
	}
}

func TestSessionSecretFromEnvironment(t *testing.T) {
	t.Setenv("SESSION_SECRET", "a-secret-of-decent-length")

	secret, generated, err := SessionSecret()
	if err != nil {
		t.Fatalf("SessionSecret() returned an unexpected error: %v", err)
	}
	if generated {
		t.Error("got generated true, want false when SESSION_SECRET is set")
	}
	if string(secret) != "a-secret-of-decent-length" {
		t.Errorf("got secret %q, want the configured value", secret)
	}
}

func TestSessionSecretIsGeneratedWhenUnset(t *testing.T) {
	t.Setenv("SESSION_SECRET", "")

	secret, generated, err := SessionSecret()
	if err != nil {
		t.Fatalf("SessionSecret() returned an unexpected error: %v", err)
	}
	if !generated {
		t.Error("got generated false, want true when SESSION_SECRET is unset")
	}
	if len(secret) != 32 {
		t.Errorf("got a %d byte secret, want 32", len(secret))
	}
}

func TestSessionSecretRejectsShortValues(t *testing.T) {
	t.Setenv("SESSION_SECRET", "tooshort")

	if _, _, err := SessionSecret(); err == nil {
		t.Error("SessionSecret() accepted an 8 character secret, want an error")
	}
}
