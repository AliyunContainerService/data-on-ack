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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/agent"
	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/event"
	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/message"
)

// --- Event envelope streamed to clients (SSE) ---

type EventType string

const (
	EventRunStarted      EventType = "run_started"
	EventThinking        EventType = "thinking"
	EventText            EventType = "text"
	EventToolCall        EventType = "tool_call"
	EventToolResult      EventType = "tool_result"
	EventConfirmRequest  EventType = "confirmation_request"
	EventConfirmResolved EventType = "confirmation_resolved"
	EventBudgetExceeded  EventType = "budget_exceeded"
	EventError           EventType = "error"
	EventDone            EventType = "done"
)

type Event struct {
	Seq  int64     `json:"seq"`
	Type EventType `json:"type"`
	Data any       `json:"data,omitempty"`
}

// --- Run states ---

type RunStatus string

const (
	RunRunning         RunStatus = "running"
	RunAwaitingConfirm RunStatus = "awaiting_confirmation"
	RunSucceeded       RunStatus = "succeeded"
	RunFailed          RunStatus = "failed"
	RunCanceled        RunStatus = "canceled"
)

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ticket is one pending HITL confirmation. The authoritative tool-call
// snapshot lives HERE, server-side: clients only ever submit
// {approved: bool}; the agent is resumed with the stored snapshot so a
// malicious client cannot confirm A and execute B (design §3.3).
type ticket struct {
	ID       string
	ReplyID  string
	ToolCall message.ToolCallBlock
	Resolved bool
	Approved bool
}

// Run is a server-side object: its lifetime is decoupled from any HTTP
// connection (design §3.1). SSE clients subscribe; disconnects do not cancel.
type Run struct {
	ID        string
	SessionID string
	User      string
	CreatedAt time.Time

	mu            sync.Mutex
	status        RunStatus
	events        []Event
	seq           int64
	subscribers   map[int64]chan Event // subscriber id -> channel
	tickets       map[string]*ticket   // cfmID -> ticket
	pending       []*ticket            // tickets of the current confirmation round
	cancel        context.CancelFunc
	submitConfirm func(*event.UserConfirmResultEvent)
	audit         *auditLogger

	lastInput string
	answerMu  sync.Mutex
	answerBuf strings.Builder
	onDone    func(*Run)

	// Budget counters (design §5.2).
	rounds    int
	toolCalls int
	costUSD   float64
}

func (r *Run) Status() RunStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *Run) emit(typ EventType, data any) {
	r.mu.Lock()
	r.seq++
	ev := Event{Seq: r.seq, Type: typ, Data: data}
	r.events = append(r.events, ev)
	for id, ch := range r.subscribers {
		select {
		case ch <- ev:
		default:
			// Slow subscriber: close its stream instead of silently dropping
			// events (review findings B8/F1). The SSE handler treats the
			// closed channel as end-of-stream; the client reconnects with
			// ?after=<lastSeq> and replays from r.events.
			delete(r.subscribers, id)
			close(ch)
		}
	}
	r.mu.Unlock()
}

// subscribe returns a channel receiving events with seq > after.
func (r *Run) subscribe(after int64) (int64, chan Event, []Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := time.Now().UnixNano()
	ch := make(chan Event, 256)
	r.subscribers[id] = ch
	var backlog []Event
	for _, ev := range r.events {
		if ev.Seq > after {
			backlog = append(backlog, ev)
		}
	}
	return id, ch, backlog
}

func (r *Run) unsubscribe(id int64) {
	r.mu.Lock()
	delete(r.subscribers, id)
	r.mu.Unlock()
}

// execute drives the agent loop. It must run in its own goroutine.
func (r *Run) execute(ag *agent.UnifiedAgent, input string, limits *Config) {
	r.lastInput = input
	ctx, cancel := context.WithTimeout(context.Background(), limits.RunTimeout)
	r.mu.Lock()
	r.cancel = cancel
	r.mu.Unlock()
	defer cancel()

	r.emit(EventRunStarted, map[string]any{"runID": r.ID})

	evCh, err := ag.ReplyStream(ctx, input)
	if err != nil {
		r.finish(RunFailed, "start agent: "+err.Error())
		return
	}

	var (
		toolNames   = map[string]string{}           // toolCallID -> name
		toolInputs  = map[string]*strings.Builder{} // toolCallID -> accumulated args JSON
		toolResults = map[string]*strings.Builder{} // toolCallID -> accumulated result text
	)

	for ev := range evCh {
		switch e := ev.(type) {
		case event.ThinkingBlockDeltaEvent:
			r.emit(EventThinking, map[string]any{"text": e.Delta})

		case event.TextBlockDeltaEvent:
			r.appendAnswer(e.Delta)
			r.emit(EventText, map[string]any{"text": e.Delta})

		case event.ToolCallStartEvent:
			toolNames[e.ToolCallID] = e.ToolCallName
			b := &strings.Builder{}
			b.WriteString(e.ToolCallInput)
			toolInputs[e.ToolCallID] = b
			r.mu.Lock()
			r.toolCalls++
			exceeded := r.toolCalls > limits.MaxToolCallsPerRun
			r.mu.Unlock()
			if exceeded {
				r.budgetExceeded(cancel, "tool call limit exceeded")
				return
			}

		case event.ToolCallDeltaEvent:
			if b, ok := toolInputs[e.ToolCallID]; ok {
				b.WriteString(e.Delta)
			}

		case event.ToolCallEndEvent:
			input := ""
			if b, ok := toolInputs[e.ToolCallID]; ok {
				input = b.String()
				delete(toolInputs, e.ToolCallID)
			}
			name := toolNames[e.ToolCallID]
			r.audit.logToolCall(r.User, r.ID, e.ToolCallID, name, input)
			r.emit(EventToolCall, map[string]any{
				"toolCallID": e.ToolCallID,
				"name":       name,
				"input":      jsonOrRaw(input),
			})

		case event.ToolResultTextDeltaEvent:
			b, ok := toolResults[e.ToolCallID]
			if !ok {
				b = &strings.Builder{}
				toolResults[e.ToolCallID] = b
			}
			b.WriteString(e.Delta)

		case event.ToolResultEndEvent:
			text := ""
			if b, ok := toolResults[e.ToolCallID]; ok {
				text = b.String()
				delete(toolResults, e.ToolCallID)
			}
			if len(text) > maxLogBytes {
				text = text[:maxLogBytes] + "\n...[truncated]..."
			}
			r.audit.logToolResult(r.User, r.ID, e.ToolCallID, string(e.State), text)
			r.emit(EventToolResult, map[string]any{
				"toolCallID": e.ToolCallID,
				"name":       toolNames[e.ToolCallID],
				"state":      string(e.State),
				"text":       text,
			})

		case event.ModelCallEndEvent:
			r.mu.Lock()
			r.rounds++
			r.costUSD += float64(e.InputTokens)/1e6*limits.PriceInputPerMillion +
				float64(e.OutputTokens)/1e6*limits.PriceOutputPerMillion
			roundsExceeded := r.rounds > limits.MaxRoundsPerRun
			cost := r.costUSD
			r.mu.Unlock()
			if roundsExceeded {
				r.budgetExceeded(cancel, "model round limit exceeded")
				return
			}
			if cost > limits.MaxDailyCostUSDPerUser {
				// Per-run approximation of the daily cap (P1: no cross-run
				// accounting store yet; see design §5.2 for the P3 ledger).
				r.budgetExceeded(cancel, fmt.Sprintf("cost limit exceeded ($%.4f)", cost))
				return
			}

		case event.RequireUserConfirmEvent:
			r.openConfirmations(e)
			// The agent blocks in waitForConfirmation until SubmitUserConfirm
			// is called (see ResolveConfirmations), with the run ctx as its
			// only timeout. Enforce the confirmation window explicitly.
			go r.confirmationWatchdog(e.ReplyID, limits.ConfirmTimeout)

		case event.ExceedMaxItersEvent:
			r.emit(EventError, map[string]any{"message": "agent exceeded max iterations"})

		case event.ReplyEndEvent:
			// handled by channel close
		}

		if ctx.Err() != nil {
			break
		}
	}

	r.mu.Lock()
	st := r.status
	r.mu.Unlock()
	if st == RunRunning || st == RunAwaitingConfirm {
		if ctx.Err() == context.Canceled {
			// No error event for user-initiated cancels; done.status=canceled
			// drives a neutral UI state (review finding F3).
			r.finish(RunCanceled, "")
		} else if ctx.Err() == context.DeadlineExceeded {
			r.finish(RunFailed, "run timeout")
		} else {
			r.finish(RunSucceeded, "")
		}
	}
}

func (r *Run) budgetExceeded(cancel context.CancelFunc, reason string) {
	r.emit(EventBudgetExceeded, map[string]any{"message": reason})
	r.audit.logBudget(r.User, r.ID, reason)
	cancel()
	r.finish(RunFailed, reason)
}

// openConfirmations creates server-side tickets for the requested tool calls.
func (r *Run) openConfirmations(e event.RequireUserConfirmEvent) {
	r.mu.Lock()
	r.pending = nil
	data := make([]map[string]any, 0, len(e.ToolCalls))
	for i := range e.ToolCalls {
		tc := e.ToolCalls[i]
		t := &ticket{ID: randHex(8), ReplyID: e.ReplyID, ToolCall: tc}
		r.tickets[t.ID] = t
		r.pending = append(r.pending, t)
		data = append(data, map[string]any{
			"confirmationID": t.ID,
			"toolCallID":     tc.ID,
			"tool":           tc.Name,
			"input":          jsonOrRaw(tc.Input),
		})
		r.audit.logConfirmRequest(r.User, r.ID, t.ID, tc.Name, tc.Input)
	}
	r.status = RunAwaitingConfirm
	r.mu.Unlock()
	r.emit(EventConfirmRequest, map[string]any{"confirmations": data})
}

// confirmationWatchdog auto-denies unresolved confirmations after the timeout.
func (r *Run) confirmationWatchdog(replyID string, timeout time.Duration) {
	time.Sleep(timeout)
	r.mu.Lock()
	stillPending := false
	for _, t := range r.pending {
		if !t.Resolved {
			stillPending = true
			break
		}
	}
	r.mu.Unlock()
	if stillPending {
		r.ResolveAllConfirmations(replyID, false)
	}
}

// ResolveConfirmation resolves ONE ticket. When every ticket of the current
// confirmation round is resolved, the agent is resumed with the STORED
// snapshots. Returns an error for unknown/already-resolved tickets.
func (r *Run) ResolveConfirmation(cfmID string, approved bool) error {
	r.mu.Lock()
	t, ok := r.tickets[cfmID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("unknown confirmation %q", cfmID)
	}
	if t.Resolved {
		r.mu.Unlock()
		return fmt.Errorf("confirmation %q already resolved", cfmID)
	}
	t.Resolved = true
	t.Approved = approved
	allResolved := true
	for _, p := range r.pending {
		if !p.Resolved {
			allResolved = false
			break
		}
	}
	var replyID string
	var results []event.ConfirmResult
	if allResolved {
		replyID = t.ReplyID
		for _, p := range r.pending {
			results = append(results, event.ConfirmResult{Confirmed: p.Approved, ToolCall: p.ToolCall})
		}
		r.pending = nil
		if r.status == RunAwaitingConfirm {
			r.status = RunRunning
		}
	}
	r.mu.Unlock()

	r.emit(EventConfirmResolved, map[string]any{"confirmationID": cfmID, "approved": approved})
	r.audit.logConfirmResolve(r.User, r.ID, cfmID, approved)

	if allResolved {
		// Resume the agent with the server-side snapshot, never with
		// client-supplied arguments. submitConfirm is bound to the session
		// agent (agent.SubmitUserConfirm) when the run starts.
		if r.submitConfirm != nil {
			// If the run already terminated (e.g. run timeout racing the
			// confirmation watchdog), nobody consumes the confirmation
			// channel anymore; submitting would block the caller forever.
			if st := r.Status(); st == RunRunning || st == RunAwaitingConfirm {
				go func() {
					res := event.NewUserConfirmResultEvent(replyID, results)
					r.submitConfirm(&res)
				}()
			}
		}
	}
	return nil
}

// ResolveAllConfirmations resolves every pending ticket of the current round
// with the same decision (used by the watchdog and by plan-level approvals).
func (r *Run) ResolveAllConfirmations(replyID string, approved bool) {
	r.mu.Lock()
	ids := make([]string, 0, len(r.pending))
	for _, t := range r.pending {
		if !t.Resolved && t.ReplyID == replyID {
			ids = append(ids, t.ID)
		}
	}
	r.mu.Unlock()
	for _, id := range ids {
		_ = r.ResolveConfirmation(id, approved)
	}
}

func (r *Run) finish(status RunStatus, errMsg string) {
	r.mu.Lock()
	if r.status == RunSucceeded || r.status == RunFailed || r.status == RunCanceled {
		r.mu.Unlock()
		return
	}
	r.status = status
	stats := map[string]any{
		"status":    string(status),
		"rounds":    r.rounds,
		"toolCalls": r.toolCalls,
		"costUSD":   r.costUSD,
	}
	r.mu.Unlock()
	if errMsg != "" {
		r.emit(EventError, map[string]any{"message": errMsg})
	}
	r.audit.logRunEnd(r.User, r.ID, stats)
	r.emit(EventDone, stats)
	if r.onDone != nil {
		go r.onDone(r)
	}
}

// appendAnswer keeps a bounded copy of the assistant's final text for the
// compacted session summary (never stored verbatim beyond the cap).
func (r *Run) appendAnswer(delta string) {
	r.answerMu.Lock()
	if r.answerBuf.Len() < 16*1024 {
		r.answerBuf.WriteString(delta)
	}
	r.answerMu.Unlock()
}

func (r *Run) answerText() string {
	r.answerMu.Lock()
	defer r.answerMu.Unlock()
	return r.answerBuf.String()
}

// --- RunRegistry ---

type RunRegistry struct {
	mu   sync.Mutex
	runs map[string]*Run
	cfg  *Config
}

func NewRunRegistry(cfg *Config) *RunRegistry {
	rr := &RunRegistry{runs: map[string]*Run{}, cfg: cfg}
	go rr.janitor()
	return rr
}

func (rr *RunRegistry) register(run *Run) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	active := 0
	for _, r := range rr.runs {
		if r.User == run.User && (r.Status() == RunRunning || r.Status() == RunAwaitingConfirm) {
			active++
		}
	}
	if active >= rr.cfg.MaxActiveRunsPerUser {
		return fmt.Errorf("too many active runs (limit %d)", rr.cfg.MaxActiveRunsPerUser)
	}
	rr.runs[run.ID] = run
	return nil
}

// hasActiveForSession reports whether the session already has a running or
// confirmation-waiting run (one active run per session, review finding B4).
func (rr *RunRegistry) hasActiveForSession(sessionID string) bool {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	for _, r := range rr.runs {
		if r.SessionID == sessionID {
			if st := r.Status(); st == RunRunning || st == RunAwaitingConfirm {
				return true
			}
		}
	}
	return false
}

func (rr *RunRegistry) get(runID, user string) (*Run, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	r, ok := rr.runs[runID]
	if !ok || r.User != user {
		return nil, fmt.Errorf("run not found")
	}
	return r, nil
}

// janitor drops finished runs after a grace period (memory hygiene, P1).
func (rr *RunRegistry) janitor() {
	for range time.Tick(10 * time.Minute) {
		rr.mu.Lock()
		for id, r := range rr.runs {
			st := r.Status()
			if (st == RunSucceeded || st == RunFailed || st == RunCanceled) &&
				time.Since(r.CreatedAt) > time.Hour {
				delete(rr.runs, id)
			}
		}
		rr.mu.Unlock()
	}
}

func newRun(sessionID, user string, audit *auditLogger) *Run {
	return &Run{
		ID:          "run-" + randHex(8),
		SessionID:   sessionID,
		User:        user,
		CreatedAt:   time.Now(),
		status:      RunRunning,
		subscribers: map[int64]chan Event{},
		tickets:     map[string]*ticket{},
		audit:       audit,
	}
}

// jsonOrRaw parses raw JSON input for display; falls back to the raw string.
func jsonOrRaw(raw string) any {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err == nil {
		return v
	}
	return raw
}
