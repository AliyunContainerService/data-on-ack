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

package service

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const testSessionNS = "kube-ai"

// newTestSessionStore builds an AgentSessionStore backed by a fake dynamic
// client. The AgentSession GVR must be registered with its list kind,
// otherwise the fake tracker cannot construct List results.
func newTestSessionStore(objs ...runtime.Object) (*AgentSessionStore, *dynamicfake.FakeDynamicClient) {
	gvr := model.AgentSessionGVR()
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{gvr: "AgentSessionList"}, objs...)
	return NewAgentSessionStore(k8s.NewClientForTesting(nil, dyn)), dyn
}

func testSession(ns, name, owner string) *model.AgentSession {
	return &model.AgentSession{
		TypeMeta:   metav1.TypeMeta{APIVersion: "data.kubeai.alibabacloud.com/v1", Kind: "AgentSession"},
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec:       model.AgentSessionSpec{OwnerUser: owner, AgentType: "copilot", Title: "title"},
	}
}

func TestAgentSessionSaveCreatesThenUpdates(t *testing.T) {
	store, dyn := newTestSessionStore()

	sess := testSession(testSessionNS, "s1", "alice")
	sess.ResourceVersion = "7"
	if err := store.Save(sess); err != nil {
		t.Fatalf("create via Save: %v", err)
	}

	got, err := store.Get(testSessionNS, "s1")
	if err != nil {
		t.Fatalf("Get after create: %v", err)
	}
	if got == nil {
		t.Fatal("Get after create returned nil session")
	}
	if got.Spec.OwnerUser != "alice" || got.Spec.AgentType != "copilot" {
		t.Fatalf("spec not persisted: %+v", got.Spec)
	}
	if got.ResourceVersion != "7" {
		t.Fatalf("resourceVersion = %q, want %q", got.ResourceVersion, "7")
	}

	// Capture the resourceVersion carried by the Update call: Save must copy
	// the existing object's resourceVersion (optimistic concurrency), not
	// send a stale/empty one.
	var updatedRV string
	dyn.PrependReactor("update", "agentsessions", func(action k8stesting.Action) (bool, runtime.Object, error) {
		u := action.(k8stesting.UpdateAction).GetObject().(*unstructured.Unstructured)
		updatedRV = u.GetResourceVersion()
		return false, nil, nil // fall through to the default tracker
	})

	sess.Spec.Title = "renamed"
	sess.Status.RunsTotal = 3
	if err := store.Save(sess); err != nil {
		t.Fatalf("update via Save: %v", err)
	}
	if updatedRV != "7" {
		t.Fatalf("Update sent resourceVersion %q, want preserved %q", updatedRV, "7")
	}

	got, err = store.Get(testSessionNS, "s1")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.Spec.Title != "renamed" || got.Status.RunsTotal != 3 {
		t.Fatalf("update not persisted: title=%q runsTotal=%d", got.Spec.Title, got.Status.RunsTotal)
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("List returned %d sessions after create+update, want 1", len(all))
	}
}

func TestAgentSessionGetNotFound(t *testing.T) {
	store, _ := newTestSessionStore()
	got, err := store.Get(testSessionNS, "missing")
	if err != nil {
		t.Fatalf("Get on missing session returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("Get on missing session = %+v, want nil", got)
	}
}

func TestAgentSessionDeleteIdempotent(t *testing.T) {
	store, _ := newTestSessionStore()

	// Delete of a never-created session must succeed.
	if err := store.Delete(testSessionNS, "nope"); err != nil {
		t.Fatalf("Delete of missing session: %v", err)
	}

	if err := store.Save(testSession(testSessionNS, "s1", "alice")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(testSessionNS, "s1"); err != nil {
		t.Fatalf("Delete existing: %v", err)
	}
	got, err := store.Get(testSessionNS, "s1")
	if err != nil || got != nil {
		t.Fatalf("Get after Delete = (%+v, %v), want (nil, nil)", got, err)
	}
	// Second Delete is a no-op success.
	if err := store.Delete(testSessionNS, "s1"); err != nil {
		t.Fatalf("second Delete: %v", err)
	}
}

func TestAgentSessionList(t *testing.T) {
	store, _ := newTestSessionStore()
	for _, s := range []*model.AgentSession{
		testSession(testSessionNS, "a", "alice"),
		testSession(testSessionNS, "b", "bob"),
		testSession("other-ns", "c", "alice"),
	} {
		if err := store.Save(s); err != nil {
			t.Fatalf("Save %s/%s: %v", s.Namespace, s.Name, err)
		}
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("List returned %d sessions, want 3", len(all))
	}

	// Callers filter by OwnerUser server-side (design §4).
	var alice []string
	for _, s := range all {
		if s.Spec.OwnerUser == "alice" {
			alice = append(alice, s.Name)
		}
	}
	if len(alice) != 2 {
		t.Fatalf("alice owns %d sessions, want 2 (%v)", len(alice), alice)
	}
}

func TestAgentSessionContextSummaryTruncated(t *testing.T) {
	store, _ := newTestSessionStore()

	sess := testSession(testSessionNS, "s1", "alice")
	sess.Status.ContextSummary = strings.Repeat("a", maxContextSummaryBytes+100)
	if err := store.Save(sess); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Get(testSessionNS, "s1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Status.ContextSummary) != maxContextSummaryBytes {
		t.Fatalf("summary length = %d, want truncated to %d",
			len(got.Status.ContextSummary), maxContextSummaryBytes)
	}
}

func TestAgentSessionContextSummaryTruncationKeepsUTF8Valid(t *testing.T) {
	store, _ := newTestSessionStore()

	// Multi-byte runes: a naive byte cut would split a rune and yield
	// invalid UTF-8, which the API server rejects.
	sess := testSession(testSessionNS, "s2", "alice")
	sess.Status.ContextSummary = strings.Repeat("中", 3000) // 9000 bytes
	if err := store.Save(sess); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Get(testSessionNS, "s2")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Status.ContextSummary) > maxContextSummaryBytes {
		t.Fatalf("summary length = %d, want <= %d",
			len(got.Status.ContextSummary), maxContextSummaryBytes)
	}
	if !utf8.ValidString(got.Status.ContextSummary) {
		t.Fatal("truncated summary is not valid UTF-8")
	}
}

func TestAgentSessionSaveRetriesOnConflict(t *testing.T) {
	store, dyn := newTestSessionStore()
	if err := store.Save(testSession(testSessionNS, "s1", "alice")); err != nil {
		t.Fatalf("initial Save: %v", err)
	}

	specUpdates := 0
	statusUpdates := 0
	dyn.PrependReactor("update", "agentsessions", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "status" {
			statusUpdates++
			return false, nil, nil
		}
		specUpdates++
		if specUpdates == 1 {
			return true, nil, apierrors.NewConflict(
				schema.GroupResource{Group: model.AgentSessionGVR().Group, Resource: model.AgentSessionGVR().Resource},
				"s1", fmt.Errorf("simulated conflict"))
		}
		return false, nil, nil
	})

	sess := testSession(testSessionNS, "s1", "alice")
	sess.Status.RunsTotal = 9
	if err := store.Save(sess); err != nil {
		t.Fatalf("Save should retry past a conflict, got: %v", err)
	}
	if specUpdates != 2 {
		t.Fatalf("spec update attempts = %d, want 2 (one conflict + one retry)", specUpdates)
	}
	if statusUpdates != 1 {
		t.Fatalf("status updates = %d, want exactly 1 (status subresource)", statusUpdates)
	}
	got, err := store.Get(testSessionNS, "s1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status.RunsTotal != 9 {
		t.Fatalf("RunsTotal = %d after conflict retry, want 9", got.Status.RunsTotal)
	}
}
