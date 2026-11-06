package auth

import "testing"

func TestUserInList(t *testing.T) {
	tests := []struct {
		user, list string
		want       bool
	}{
		{"alice", "alice,bob", true},
		{"bob", " alice , bob ", true},
		{"carol", "alice,bob", false},
		{"alice", "", false},
		{"", "alice", false},
	}
	for _, tt := range tests {
		if got := userInList(tt.user, tt.list); got != tt.want {
			t.Errorf("userInList(%q, %q) = %v, want %v", tt.user, tt.list, got, tt.want)
		}
	}
}

func TestGroupsIntersect(t *testing.T) {
	tests := []struct {
		header, adminGroups string
		want                bool
	}{
		{"dev,ops", "ops", true},
		{"dev,ops", "ops,admin", true},
		{"dev", "ops,admin", false},
		{"dev", "", false},
		{"", "ops", false},
	}
	for _, tt := range tests {
		if got := groupsIntersect(tt.header, tt.adminGroups); got != tt.want {
			t.Errorf("groupsIntersect(%q, %q) = %v, want %v", tt.header, tt.adminGroups, got, tt.want)
		}
	}
}

func TestNoneModeNamespacesDefault(t *testing.T) {
	// Without config initialized, the built-in default list is returned.
	got := NoneModeNamespaces()
	if len(got) == 0 {
		t.Fatal("NoneModeNamespaces returned empty list")
	}
	if got[0] != "default" {
		t.Errorf("NoneModeNamespaces()[0] = %q, want %q", got[0], "default")
	}
}

func TestCurrentModeDefault(t *testing.T) {
	// Without config initialized, the mode defaults to aliyun.
	if got := CurrentMode(); got != ModeAliyun {
		t.Errorf("CurrentMode() = %q, want %q", got, ModeAliyun)
	}
	if AuthDisabled() {
		t.Error("AuthDisabled() = true, want false")
	}
}
