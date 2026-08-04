package service

import (
	"testing"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/model"
)

func TestBuildNodeName(t *testing.T) {
	tests := []struct {
		parent string
		child  string
		want   string
	}{
		{"", "root", "root"},
		{"root", "default", "root.default"},
		{"root.default", "groupA", "root.default.groupA"},
	}
	for _, tt := range tests {
		got := buildNodeName(tt.parent, tt.child)
		if got != tt.want {
			t.Errorf("buildNodeName(%q, %q) = %q, want %q", tt.parent, tt.child, got, tt.want)
		}
	}
}

func TestSerializeNodeName(t *testing.T) {
	got := SerializeNodeName("root.default", "child")
	if got != "root.default.child" {
		t.Errorf("SerializeNodeName = %q, want %q", got, "root.default.child")
	}

	got = SerializeNodeName("", "root")
	if got != "root" {
		t.Errorf("SerializeNodeName with empty prefix = %q, want %q", got, "root")
	}
}

func TestDeserializeNodeName(t *testing.T) {
	tests := []struct {
		fullName   string
		wantPrefix string
		wantName   string
	}{
		{"root.default.groupA", "root.default", "groupA"},
		{"root", "", "root"},
		{"single", "", "single"},
	}
	for _, tt := range tests {
		prefix, name := DeserializeNodeName(tt.fullName)
		if prefix != tt.wantPrefix {
			t.Errorf("DeserializeNodeName(%q) prefix = %q, want %q", tt.fullName, prefix, tt.wantPrefix)
		}
		if name != tt.wantName {
			t.Errorf("DeserializeNodeName(%q) name = %q, want %q", tt.fullName, name, tt.wantName)
		}
	}
}

func TestRemoveNodeFromTree(t *testing.T) {
	root := &model.ElasticQuotaNode{
		Name: "root",
		Children: []*model.ElasticQuotaNode{
			{Name: "root.a"},
			{Name: "root.b"},
			{Name: "root.c"},
		},
	}

	// Remove middle child
	if !removeNodeFromTree(root, "root.b") {
		t.Fatal("removeNodeFromTree returned false")
	}
	if len(root.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(root.Children))
	}
	if root.Children[0].Name != "root.a" || root.Children[1].Name != "root.c" {
		t.Errorf("remaining children = %v", root.Children)
	}

	// Remove non-existent
	if removeNodeFromTree(root, "nonexistent") {
		t.Error("should return false for non-existent node")
	}

	// Remove nested child
	root.Children[0].Children = []*model.ElasticQuotaNode{
		{Name: "root.a.leaf1"},
		{Name: "root.a.leaf2"},
	}
	if !removeNodeFromTree(root, "root.a.leaf1") {
		t.Fatal("failed to remove nested child")
	}
	if len(root.Children[0].Children) != 1 {
		t.Fatalf("expected 1 nested child, got %d", len(root.Children[0].Children))
	}
}

func TestUniqueStrings(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b", "d"}
	result := uniqueStrings(input)
	if len(result) != 4 {
		t.Fatalf("expected 4 unique, got %d: %v", len(result), result)
	}
	expected := map[string]bool{"a": true, "b": true, "c": true, "d": true}
	for _, s := range result {
		if !expected[s] {
			t.Errorf("unexpected: %s", s)
		}
	}
}

func TestContains(t *testing.T) {
	slice := []string{"admin", "researcher"}
	if !contains(slice, "admin") {
		t.Error("should contain admin")
	}
	if contains(slice, "unknown") {
		t.Error("should not contain unknown")
	}
	if contains([]string{}, "anything") {
		t.Error("empty slice should not contain anything")
	}
}

func TestNormalizeUserID_ServiceLevel(t *testing.T) {
	// Test the service-level normalizeUserID (same as auth.NormalizeUserID)
	tests := []struct {
		input string
		want  string
	}{
		{"user@example.com", "user-example.com"},
		{"Foo_Bar", "foo-bar"},
	}
	for _, tt := range tests {
		got := normalizeUserID(tt.input)
		if got != tt.want {
			t.Errorf("normalizeUserID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
