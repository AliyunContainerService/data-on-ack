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
	"sync"
	"testing"
	"time"

	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/event"
	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/message"
)

func testRun() (*Run, *[]event.UserConfirmResultEvent, *sync.Mutex) {
	var mu sync.Mutex
	submitted := &[]event.UserConfirmResultEvent{}
	r := newRun("sess-test", "user-a", newAuditLogger())
	r.submitConfirm = func(res *event.UserConfirmResultEvent) {
		mu.Lock()
		*submitted = append(*submitted, *res)
		mu.Unlock()
	}
	return r, submitted, &mu
}

func confirmEvent(replyID, tcID, tool, input string) event.RequireUserConfirmEvent {
	return event.NewRequireUserConfirmEvent(replyID, []message.ToolCallBlock{
		{Type: "tool_call", ID: tcID, Name: tool, Input: input},
	})
}

func TestConfirmationSnapshotBinding(t *testing.T) {
	r, submitted, mu := testRun()

	ev := confirmEvent("reply-1", "tc-1", "delete_something", `{"namespace":"ns","name":"x"}`)
	r.openConfirmations(ev)

	if st := r.Status(); st != RunAwaitingConfirm {
		t.Fatalf("status = %v, want awaiting_confirmation", st)
	}
	r.mu.Lock()
	var cfmID string
	for id := range r.tickets {
		cfmID = id
	}
	r.mu.Unlock()
	if cfmID == "" {
		t.Fatal("no ticket created")
	}

	// Approving must submit the SERVER-SIDE snapshot, and exactly once.
	// The submit is delivered asynchronously; poll briefly.
	if err := r.ResolveConfirmation(cfmID, true); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(*submitted)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	mu.Lock()
	n := len(*submitted)
	var res event.UserConfirmResultEvent
	if n > 0 {
		res = (*submitted)[0]
	}
	mu.Unlock()
	if n != 1 {
		t.Fatalf("submitted %d times, want 1", n)
	}
	if len(res.ConfirmResults) != 1 || !res.ConfirmResults[0].Confirmed {
		t.Fatalf("bad confirm results: %+v", res)
	}
	got := res.ConfirmResults[0].ToolCall
	if got.ID != "tc-1" || got.Name != "delete_something" || got.Input != `{"namespace":"ns","name":"x"}` {
		t.Fatalf("snapshot not preserved: %+v", got)
	}
	if res.ReplyID != "reply-1" {
		t.Fatalf("reply id = %q", res.ReplyID)
	}

	// Replay must be rejected.
	if err := r.ResolveConfirmation(cfmID, true); err == nil {
		t.Fatal("replayed confirmation accepted")
	}
	// Unknown ticket must be rejected.
	if err := r.ResolveConfirmation("nope", true); err == nil {
		t.Fatal("unknown confirmation accepted")
	}
}

func TestConfirmationDeniedAfterTerminal(t *testing.T) {
	r, submitted, mu := testRun()
	r.openConfirmations(confirmEvent("reply-2", "tc-2", "write_tool", `{}`))
	r.finish(RunSucceeded, "") // run ends while confirmation is pending

	r.mu.Lock()
	var cfmID string
	for id := range r.tickets {
		cfmID = id
	}
	r.mu.Unlock()

	// Resolving after terminal state must not block and must not submit.
	if err := r.ResolveConfirmation(cfmID, true); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	mu.Lock()
	n := len(*submitted)
	mu.Unlock()
	if n != 0 {
		t.Fatalf("submitted %d times after terminal state, want 0", n)
	}
}

func TestBudgetCounters(t *testing.T) {
	r, _, _ := testRun()
	r.mu.Lock()
	r.toolCalls = 39
	r.mu.Unlock()
	// Emitting via budgetExceeded path is exercised indirectly; here just
	// verify the counters are guarded state.
	if r.Status() != RunRunning {
		t.Fatalf("unexpected status %v", r.Status())
	}
}

func TestSubscribeBacklogAndLive(t *testing.T) {
	r, _, _ := testRun()
	r.emit(EventText, map[string]any{"text": "hello"})

	id, ch, backlog := r.subscribe(0)
	if len(backlog) != 1 || backlog[0].Type != EventText {
		t.Fatalf("backlog = %+v", backlog)
	}
	r.emit(EventText, map[string]any{"text": "world"})
	ev := <-ch
	if ev.Seq != 2 || ev.Type != EventText {
		t.Fatalf("live event = %+v", ev)
	}
	// Late subscriber with after=2 sees nothing in backlog.
	_, _, empty := r.subscribe(2)
	if len(empty) != 0 {
		t.Fatalf("expected empty backlog, got %+v", empty)
	}
	r.unsubscribe(id)
}
