package auth

import (
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/config"
)

// Supported auth modes (AppConfig.AuthMode).
const (
	// ModeAliyun authenticates users with Alibaba Cloud RAM OAuth2 SSO.
	ModeAliyun = "aliyun"
	// ModeNone disables authentication entirely: every request is treated
	// as the built-in admin user with full permissions.
	ModeNone = "none"
	// ModeCustom trusts identity headers injected by an upstream gateway,
	// allowing integration with a customer's own user system.
	ModeCustom = "custom"
)

// NoneModeUser is the built-in identity used when auth is disabled.
const NoneModeUser = "admin"

// CurrentMode returns the configured auth mode, defaulting to ModeAliyun
// when the config has not been initialized (e.g. in unit tests).
func CurrentMode() string {
	cfg := config.Get()
	if cfg == nil || cfg.AuthMode == "" {
		return ModeAliyun
	}
	return cfg.AuthMode
}

// AuthDisabled reports whether authentication is fully disabled (mode "none").
func AuthDisabled() bool {
	return CurrentMode() == ModeNone
}

// NoneModeNamespaces returns the namespace scope for list endpoints when
// auth is disabled.
func NoneModeNamespaces() []string {
	cfg := config.Get()
	raw := "default,default-group,kube-ai"
	if cfg != nil && cfg.NoneModeNamespaces != "" {
		raw = cfg.NoneModeNamespaces
	}
	var namespaces []string
	for _, ns := range strings.Split(raw, ",") {
		if ns = strings.TrimSpace(ns); ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	if len(namespaces) == 0 {
		namespaces = []string{"default"}
	}
	return namespaces
}

// userInList reports whether user appears in a comma-separated list.
func userInList(user, csvList string) bool {
	for _, item := range strings.Split(csvList, ",") {
		if strings.TrimSpace(item) != "" && strings.TrimSpace(item) == user {
			return true
		}
	}
	return false
}

// groupsIntersect reports whether any group in the header value (comma-separated)
// appears in the configured comma-separated admin group list.
func groupsIntersect(headerGroups, adminGroups string) bool {
	if strings.TrimSpace(adminGroups) == "" {
		return false
	}
	for _, g := range strings.Split(headerGroups, ",") {
		if userInList(strings.TrimSpace(g), adminGroups) {
			return true
		}
	}
	return false
}
