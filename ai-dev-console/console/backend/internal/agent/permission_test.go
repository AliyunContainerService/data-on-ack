package agent

import (
	"context"
	"testing"

	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/permission"
	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/tool"
)

// buildPolicyContext mirrors the wiring in agent.go / persist.go.
func buildPolicyContext() *permission.Context {
	pctx := permission.NewContext(permission.ModeBypass)
	for _, name := range WriteToolNames {
		pctx.AskRules[name] = append(pctx.AskRules[name], permission.Rule{
			ToolName: name, Behavior: permission.BehaviorAsk, Source: "console-agent-policy",
		})
	}
	return pctx
}

func TestWriteToolsRequireConfirmation(t *testing.T) {
	eng := permission.NewEngine(buildPolicyContext())

	for _, name := range WriteToolNames {
		tk := tool.NewFunctionTool(name, "t", []byte(`{"type":"object"}`),
			func(ctx context.Context, input map[string]any) (any, error) { return nil, nil })
		d, err := eng.CheckPermission(tk, map[string]any{})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if d.Behavior != permission.BehaviorAsk {
			t.Fatalf("%s behavior = %v, want ask", name, d.Behavior)
		}
	}

	// Read tools must flow through without confirmation in bypass mode.
	readTool := tool.NewFunctionTool("list_training_jobs", "t", []byte(`{"type":"object"}`),
		func(ctx context.Context, input map[string]any) (any, error) { return nil, nil })
	d, err := eng.CheckPermission(readTool, map[string]any{})
	if err != nil {
		t.Fatalf("read tool: %v", err)
	}
	if d.Behavior == permission.BehaviorAsk {
		t.Fatalf("read tool must not require confirmation, got %v", d.Behavior)
	}
}

func TestIsWriteTool(t *testing.T) {
	for _, name := range WriteToolNames {
		if !isWriteTool(name) {
			t.Fatalf("%s should be a write tool", name)
		}
	}
	if isWriteTool("get_job_logs") {
		t.Fatal("get_job_logs is read-only")
	}
}
