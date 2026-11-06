package config

import (
	"os"
	"testing"
)

func TestResolveAuthMode(t *testing.T) {
	tests := []struct {
		name        string
		flagVal     string
		envVal      string
		disableAuth bool
		want        string
	}{
		{"flag wins", "custom", "none", false, "custom"},
		{"flag only", "none", "", false, "none"},
		{"env", "", "custom", false, "custom"},
		{"env case-insensitive", "", "NONE", false, "none"},
		{"legacy DISABLE_AUTH", "", "", true, "none"},
		{"env wins over DISABLE_AUTH", "", "custom", true, "custom"},
		{"default", "", "", false, "aliyun"},
		{"unknown falls back", "", "oauth2", false, "aliyun"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("AUTH_MODE")
			os.Unsetenv("DISABLE_AUTH")
			if tt.envVal != "" {
				os.Setenv("AUTH_MODE", tt.envVal)
			}
			if tt.disableAuth {
				os.Setenv("DISABLE_AUTH", "true")
			}
			defer os.Unsetenv("AUTH_MODE")
			defer os.Unsetenv("DISABLE_AUTH")
			if got := resolveAuthMode(tt.flagVal); got != tt.want {
				t.Errorf("resolveAuthMode(%q) = %q, want %q", tt.flagVal, got, tt.want)
			}
		})
	}
}
