package credential

import (
	"sync"
	"time"
)

// AKInfo holds Alibaba Cloud access credentials.
type AKInfo struct {
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
	Expiration      string // RFC3339 / ISO8601
}

// Provider is the interface for credential sources.
type Provider interface {
	GetAKInfo() (*AKInfo, error)
}

// Manager wraps a Provider with caching and auto-refresh.
type Manager struct {
	provider Provider
	mu       sync.RWMutex
	cached   *AKInfo
}

func NewManager(provider Provider) *Manager {
	return &Manager{provider: provider}
}

// Get returns cached credentials, refreshing if expired.
func (m *Manager) Get() (*AKInfo, error) {
	m.mu.RLock()
	cached := m.cached
	m.mu.RUnlock()

	if cached != nil && !isExpired(cached) {
		return cached, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if m.cached != nil && !isExpired(m.cached) {
		return m.cached, nil
	}

	ak, err := m.provider.GetAKInfo()
	if err != nil {
		return nil, err
	}
	m.cached = ak
	return ak, nil
}

// ForceRefresh clears the cache, forcing a refresh on next Get().
func (m *Manager) ForceRefresh() {
	m.mu.Lock()
	m.cached = nil
	m.mu.Unlock()
}

func isExpired(ak *AKInfo) bool {
	if ak.Expiration == "" {
		return false // static keys never expire
	}
	layout := "2006-01-02T15:04:05Z"
	t, err := time.Parse(layout, ak.Expiration)
	if err != nil {
		return true
	}
	return t.Before(time.Now())
}
