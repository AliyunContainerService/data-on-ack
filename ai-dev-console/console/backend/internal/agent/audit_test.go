package agent

import (
	"strings"
	"testing"
)

func TestSanitizeArgsRecursive(t *testing.T) {
	in := `{"name":"job1","config":{"token":"abc","deep":{"password":"p"}},"list":[{"apiKey":"k"},1]}`
	out, ok := sanitizeArgs(in).(map[string]any)
	if !ok {
		t.Fatal("expected map")
	}
	cfg := out["config"].(map[string]any)
	if cfg["token"] != "[redacted]" {
		t.Fatalf("nested token not redacted: %v", cfg["token"])
	}
	deep := cfg["deep"].(map[string]any)
	if deep["password"] != "[redacted]" {
		t.Fatalf("deep password not redacted: %v", deep["password"])
	}
	list := out["list"].([]any)
	item := list[0].(map[string]any)
	if item["apiKey"] != "[redacted]" {
		t.Fatalf("apiKey in array not redacted: %v", item["apiKey"])
	}
	if out["name"] != "job1" {
		t.Fatalf("non-sensitive value altered: %v", out["name"])
	}
	if strings.Contains(in, "unparsable") {
		t.Fatal("unexpected")
	}
}

func TestAuditHashChain(t *testing.T) {
	a := newAuditLogger()
	a.record("u", "r1", "tool_call", map[string]any{"tool": "x"})
	a.record("u", "r1", "run_end", map[string]any{"status": "succeeded"})
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.entries) != 2 {
		t.Fatalf("entries = %d", len(a.entries))
	}
	if a.entries[0].Hash == "" || a.entries[1].Hash == "" || a.entries[0].Hash == a.entries[1].Hash {
		t.Fatal("hash chain broken")
	}
}
