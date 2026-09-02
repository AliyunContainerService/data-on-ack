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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	asagent "github.com/alanfokco/agentscope-go/v2/pkg/agentscope/agent"
	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/message"
	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/permission"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// summaryPartBytes caps each component of the compacted context summary.
const summaryPartBytes = 2048

// persistSession saves the session metadata CR (best-effort; the in-memory
// session is authoritative while the backend runs).
func (m *Manager) persistSession(s *Session) {
	if m.store == nil {
		return
	}
	cr := &model.AgentSession{
		ObjectMeta: metav1.ObjectMeta{Namespace: s.Namespace, Name: s.Name},
		Spec: model.AgentSessionSpec{
			OwnerUser: s.User,
			AgentType: string(s.Type),
			Title:     s.Title,
			Locale:    s.Locale,
		},
		Status: model.AgentSessionStatus{
			LastActive: metav1.Time{Time: s.LastActive},
		},
	}
	if old, err := m.store.Get(s.Namespace, s.Name); err == nil && old != nil {
		cr.Status = old.Status // keep summary/runs; refresh below on run end
	}
	if err := m.store.Save(cr); err != nil {
		logrus.Warnf("persist agent session %s/%s: %v", s.Namespace, s.Name, err)
	}
}

func (m *Manager) deleteSessionCR(namespace, name string) {
	if m.store == nil {
		return
	}
	if err := m.store.Delete(namespace, name); err != nil {
		logrus.Warnf("delete agent session CR %s/%s: %v", namespace, name, err)
	}
}

// onRunDone is wired as Run.onDone: it compacts the conversation into the
// session CR status (metadata + bounded summary only; raw streams never
// enter etcd, design §3.1).
func (m *Manager) onRunDone(s *Session, r *Run) {
	if m.store == nil {
		return
	}
	cr, err := m.store.Get(s.Namespace, s.Name)
	if err != nil {
		logrus.Warnf("load agent session CR for run done: %v", err)
		return
	}
	if cr == nil {
		return
	}
	cr.Status.ContextSummary = compactSummary(r.lastInput, r.answerText())
	cr.Status.LastActive = metav1.Time{Time: time.Now()}
	cr.Status.RunsTotal++
	if err := m.store.Save(cr); err != nil {
		logrus.Warnf("update agent session CR after run: %v", err)
	}
	s.mu.Lock()
	s.LastActive = time.Now()
	s.mu.Unlock()
}

// compactSummary bounds the stored context: last user message + final answer.
func compactSummary(userMsg, answer string) string {
	clip := func(s string) string {
		if len(s) > summaryPartBytes {
			return s[:summaryPartBytes] + "...[truncated]"
		}
		return s
	}
	return fmt.Sprintf("Last user message:\n%s\n\nAssistant final answer:\n%s", clip(userMsg), clip(answer))
}

// RestoreSessions reloads persisted sessions after a backend restart
// (design §3.1 "restart recovery"). Conversation memory starts fresh but is
// primed with the stored compacted summary via Observe.
func (m *Manager) RestoreSessions() {
	if m.store == nil {
		return
	}
	items, err := m.store.List()
	if err != nil {
		logrus.Warnf("restore agent sessions: %v", err)
		return
	}
	restored := 0
	for i := range items {
		cr := &items[i]
		if m.buildRestoredSession(cr) {
			restored++
		}
	}
	if restored > 0 {
		logrus.Infof("restored %d agent sessions from CRs", restored)
	}
}

func (m *Manager) buildRestoredSession(cr *model.AgentSession) bool {
	if cr.Spec.OwnerUser == "" {
		return false
	}
	typ := AgentType(cr.Spec.AgentType)
	if typ != AgentCopilot && typ != AgentDiagnose {
		typ = AgentCopilot
	}

	// Restored sessions have no live namespace scope; the scope is re-derived
	// from the login session on first use, so tools stay safe: an empty scope
	// means allowNS rejects everything until the user interacts again.
	env := &toolEnv{
		UserName:          cr.Spec.OwnerUser,
		AllowedNamespaces: nil,
		Notebooks:         m.Notebooks,
		Training:          m.Training,
		Serving:           m.Serving,
		Metrics:           m.Metrics,
		Quota:             m.Quota,
	}

	tk := NewReadOnlyToolkit(env)
	tk.AddGroup("write", NewWriteToolkit(env)...)
	pctx := permission.NewContext(permission.ModeBypass)
	for _, name := range WriteToolNames {
		pctx.AskRules[name] = append(pctx.AskRules[name], permission.Rule{
			ToolName: name, Behavior: permission.BehaviorAsk, Source: "console-agent-policy",
		})
	}

	ag := asagent.NewUnifiedAgent(
		"console-"+string(typ),
		systemPrompt(typ, cr.Spec.OwnerUser, []string{}, cr.Spec.Locale),
		m.model,
		asagent.WithToolkit(tk),
		asagent.WithReactConfig(asagent.ReactConfig{MaxIters: m.cfg.MaxRoundsPerRun}),
		asagent.WithPermissionContext(pctx),
	)

	// Prime with the compacted summary (clearly labeled as history).
	if cr.Status.ContextSummary != "" {
		_ = ag.Observe(context.Background(), []*message.Msg{
			message.NewMsg("system-history", message.RoleSystem,
				"[Restored context summary from a previous backend lifetime]\n"+cr.Status.ContextSummary),
		})
	}

	s := &Session{
		ID:         "sess-" + randHex(8),
		Namespace:  cr.Namespace,
		Name:       cr.Name,
		User:       cr.Spec.OwnerUser,
		Type:       typ,
		Title:      cr.Spec.Title,
		Locale:     cr.Spec.Locale,
		CreatedAt:  cr.CreationTimestamp.Time,
		LastActive: time.Now(),
		agent:      ag,
		env:        env,
	}
	m.sessions.Store(s.ID, s)
	return true
}

var _ = strings.TrimSpace
