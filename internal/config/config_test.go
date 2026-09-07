package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewEmptyConfigProvidesUsableDefaults(t *testing.T) {
	config := NewEmptyConfig()
	if config.Dash.AuthJWT == "" || config.Dash.RefreshJWT == "" {
		t.Fatal("generated authentication secrets must be non-empty")
	}
	if config.Dash.AuthJWT == config.Dash.RefreshJWT {
		t.Fatal("generated authentication secrets must be independent")
	}
	if config.Database.AutoMigrateDrop {
		t.Fatal("destructive database migration must be disabled by default")
	}
}

func TestNewConfigValidatesRequiredSecrets(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "missing dashboard secrets",
			content: `{}`,
			wantErr: "dash auth_jwt and refresh_jwt must be non-empty",
		},
		{
			name:    "missing refresh secret",
			content: `{"dash":{"auth_jwt":"auth-secret"}}`,
			wantErr: "dash auth_jwt and refresh_jwt must be non-empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConfig(writeConfig(t, tt.content))
			if err == nil {
				t.Fatalf("NewConfig() error = nil, want %q", tt.wantErr)
			}
			if got := err.Error(); got != tt.wantErr {
				t.Errorf("NewConfig() error = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestNewConfigAppliesLogLevelDefault(t *testing.T) {
	config, err := NewConfig(writeConfig(t, `{
		"dash":{"auth_jwt":"auth-secret","refresh_jwt":"refresh-secret"}
	}`))
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}
	if got, want := config.Log.Level, "info"; got != want {
		t.Errorf("Log.Level = %q, want %q", got, want)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
