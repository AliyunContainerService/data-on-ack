/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
limitations under the License.
*/

package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const maxAuditEntries = 10000

type auditEntry struct {
	Time   time.Time      `json:"time"`
	User   string         `json:"user"`
	RunID  string         `json:"run_id"`
	Action string         `json:"action"`
	Detail map[string]any `json:"detail,omitempty"`
	Hash   string         `json:"hash"` // sha256(prevHash + canonical(entry))
}

// auditLogger is an in-memory, hash-chained audit trail (design §5.2:
// sanitized full records + hash chain, not "hashes only"). Entries are also
// mirrored to the application log for external collection.
type auditLogger struct {
	mu      sync.Mutex
	prev    string
	entries []auditEntry
}

func newAuditLogger() *auditLogger {
	return &auditLogger{prev: "genesis"}
}

func (a *auditLogger) record(user, runID, action string, detail map[string]any) {
	a.mu.Lock()
	e := auditEntry{Time: time.Now(), User: user, RunID: runID, Action: action, Detail: detail}
	canonical, _ := json.Marshal(e)
	sum := sha256.Sum256([]byte(a.prev + string(canonical)))
	e.Hash = hex.EncodeToString(sum[:])
	a.prev = e.Hash
	a.entries = append(a.entries, e)
	if len(a.entries) > maxAuditEntries {
		a.entries = a.entries[len(a.entries)-maxAuditEntries:]
	}
	a.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"agent_audit": true, "user": user, "run": runID, "action": action, "hash": e.Hash,
	}).Info("agent audit")
}

// sanitizeArgs removes values of keys that commonly carry credentials before
// anything is persisted.
func sanitizeArgs(raw string) any {
	var v map[string]any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return "[unparsable]"
	}
	for k := range v {
		switch k {
		case "password", "token", "apiKey", "api_key", "secret", "accessKey", "access_key":
			v[k] = "[redacted]"
		}
	}
	return v
}

func (a *auditLogger) logToolCall(user, runID, toolCallID, tool, argsJSON string) {
	a.record(user, runID, "tool_call", map[string]any{
		"toolCallID": toolCallID, "tool": tool, "args": sanitizeArgs(argsJSON),
	})
}

func (a *auditLogger) logToolResult(user, runID, toolCallID, state, text string) {
	preview := text
	if len(preview) > 512 {
		preview = preview[:512] + "..."
	}
	a.record(user, runID, "tool_result", map[string]any{
		"toolCallID": toolCallID, "state": state, "preview": preview,
	})
}

func (a *auditLogger) logConfirmRequest(user, runID, cfmID, tool, argsJSON string) {
	a.record(user, runID, "confirm_request", map[string]any{
		"confirmationID": cfmID, "tool": tool, "args": sanitizeArgs(argsJSON),
	})
}

func (a *auditLogger) logConfirmResolve(user, runID, cfmID string, approved bool) {
	a.record(user, runID, "confirm_resolve", map[string]any{
		"confirmationID": cfmID, "approved": approved,
	})
}

func (a *auditLogger) logBudget(user, runID, reason string) {
	a.record(user, runID, "budget_exceeded", map[string]any{"reason": reason})
}

func (a *auditLogger) logRunEnd(user, runID string, stats map[string]any) {
	a.record(user, runID, "run_end", stats)
}
