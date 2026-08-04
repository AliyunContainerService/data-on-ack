package credential

import (
	"testing"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/config"
)

func TestStaticProvider(t *testing.T) {
	p := &StaticProvider{
		accessKeyID:     "test-ak-id",
		accessKeySecret: "test-ak-secret",
	}
	ak, err := p.GetAKInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ak.AccessKeyID != "test-ak-id" {
		t.Errorf("AccessKeyID = %q, want %q", ak.AccessKeyID, "test-ak-id")
	}
	if ak.AccessKeySecret != "test-ak-secret" {
		t.Errorf("AccessKeySecret = %q, want %q", ak.AccessKeySecret, "test-ak-secret")
	}
	if ak.SecurityToken != "" {
		t.Errorf("SecurityToken should be empty for static, got %q", ak.SecurityToken)
	}
	if ak.Expiration != "" {
		t.Errorf("Expiration should be empty for static, got %q", ak.Expiration)
	}
}

func TestNewStaticProvider_MissingCreds(t *testing.T) {
	cfg := &config.AppConfig{
		CredentialMode:   "static",
		AccessKeyID:     "",
		AccessKeySecret: "",
	}
	_, err := NewStaticProvider(cfg)
	if err == nil {
		t.Fatal("expected error when AK env vars are missing")
	}
}

func TestIsExpired_EmptyExpiration(t *testing.T) {
	ak := &AKInfo{AccessKeyID: "id", Expiration: ""}
	if isExpired(ak) {
		t.Error("static credentials should not be expired")
	}
}

func TestIsExpired_PastTime(t *testing.T) {
	ak := &AKInfo{
		Expiration: time.Now().UTC().Add(-1 * time.Hour).Format("2006-01-02T15:04:05Z"),
	}
	if !isExpired(ak) {
		t.Error("past expiration should be expired")
	}
}

func TestIsExpired_FutureTime(t *testing.T) {
	ak := &AKInfo{
		Expiration: time.Now().UTC().Add(1 * time.Hour).Format("2006-01-02T15:04:05Z"),
	}
	if isExpired(ak) {
		t.Error("future expiration should not be expired")
	}
}

func TestIsExpired_InvalidFormat(t *testing.T) {
	ak := &AKInfo{Expiration: "not-a-date"}
	if !isExpired(ak) {
		t.Error("invalid expiration format should be treated as expired")
	}
}

func TestManager_Caching(t *testing.T) {
	p := &mockProvider{}
	m := NewManager(p)

	ak1, err := m.Get()
	if err != nil {
		t.Fatalf("first Get error: %v", err)
	}
	if p.callCount != 1 {
		t.Errorf("provider callCount = %d, want 1", p.callCount)
	}

	// Second call should use cache (static = never expires)
	_, _ = m.Get()
	if p.callCount != 1 {
		t.Errorf("provider callCount = %d, want 1 (cached)", p.callCount)
	}

	// ForceRefresh clears cache
	m.ForceRefresh()
	_, _ = m.Get()
	if p.callCount != 2 {
		t.Errorf("provider callCount = %d, want 2 after refresh", p.callCount)
	}

	if ak1 == nil {
		t.Error("ak1 should not be nil")
	}
}

// --- mocks ---

type mockProvider struct {
	callCount int
}

func (m *mockProvider) GetAKInfo() (*AKInfo, error) {
	m.callCount++
	return &AKInfo{AccessKeyID: "mock-ak", AccessKeySecret: "mock-sk"}, nil
}
