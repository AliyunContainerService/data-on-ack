package config

import (
	"os"
	"testing"
)

func TestResolveCredMode(t *testing.T) {
	tests := []struct {
		flagVal string
		envVal  string
		want    string
	}{
		{"static", "", "static"},
		{"rrsa", "", "rrsa"},
		{"", "rrsa", "rrsa"},
		{"", "static", "static"},
		{"", "", "static"}, // default
	}
	for _, tt := range tests {
		t.Run(tt.flagVal+"/"+tt.envVal, func(t *testing.T) {
			os.Unsetenv("CREDENTIAL_MODE")
			if tt.envVal != "" {
				os.Setenv("CREDENTIAL_MODE", tt.envVal)
			}
			got := resolveCredMode(tt.flagVal)
			if got != tt.want {
				t.Errorf("resolveCredMode(%q) = %q, want %q", tt.flagVal, got, tt.want)
			}
		})
	}
	os.Unsetenv("CREDENTIAL_MODE")
}

func TestGetEnvDefault(t *testing.T) {
	os.Unsetenv("TEST_KEY")
	got := getEnvDefault("TEST_KEY", "fallback")
	if got != "fallback" {
		t.Errorf("getEnvDefault with unset env = %q, want %q", got, "fallback")
	}

	os.Setenv("TEST_KEY", "env-value")
	got = getEnvDefault("TEST_KEY", "fallback")
	if got != "env-value" {
		t.Errorf("getEnvDefault with set env = %q, want %q", got, "env-value")
	}
	os.Unsetenv("TEST_KEY")
}

func TestParse(t *testing.T) {
	// Test that Parse sets up config correctly
	os.Setenv("AK_ACCESS_KEY_ID", "test-ak")
	os.Setenv("AK_ACCESS_KEY_SECRET", "test-sk")
	defer func() {
		os.Unsetenv("AK_ACCESS_KEY_ID")
		os.Unsetenv("AK_ACCESS_KEY_SECRET")
	}()

	Parse()
	cfg := Get()
	if cfg == nil {
		t.Fatal("Get() returned nil after Parse()")
	}
	if cfg.AccessKeyID != "test-ak" {
		t.Errorf("AccessKeyID = %q, want %q", cfg.AccessKeyID, "test-ak")
	}
	if cfg.AccessKeySecret != "test-sk" {
		t.Errorf("AccessKeySecret = %q, want %q", cfg.AccessKeySecret, "test-sk")
	}
	if cfg.CredentialMode != "static" {
		t.Errorf("CredentialMode = %q, want %q", cfg.CredentialMode, "static")
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":8080")
	}
}

func TestRamSigninURL(t *testing.T) {
	cfg := &AppConfig{IsIntlAccount: false}
	if got := cfg.RamSigninURL(); got != "https://signin.aliyun.com/oauth2/v1/auth" {
		t.Errorf("domestic signin URL = %q", got)
	}

	cfg.IsIntlAccount = true
	if got := cfg.RamSigninURL(); got != "https://signin.alibabacloud.com/oauth2/v1/auth" {
		t.Errorf("intl signin URL = %q", got)
	}
}

func TestRamTokenURL(t *testing.T) {
	cfg := &AppConfig{IsIntlAccount: false}
	got := cfg.RamTokenURL()
	// VPC proxy domain or fallback — both contain "/v1/token"
	if got == "" {
		t.Error("RamTokenURL should not be empty")
	}
}

func TestRamIMSDomain(t *testing.T) {
	cfg := &AppConfig{}
	got := cfg.RamIMSDomain()
	if got == "" {
		t.Error("RamIMSDomain should not be empty")
	}
}
