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
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// maxContextSummaryBytes bounds the compacted context stored per session so a
// long conversation can never pressure etcd (raw streams stay out by design).
const maxContextSummaryBytes = 8192

// AgentSessionStore persists AgentSession CRs via the dynamic client.
type AgentSessionStore struct {
	client *k8s.Client
}

func NewAgentSessionStore(client *k8s.Client) *AgentSessionStore {
	return &AgentSessionStore{client: client}
}

func (s *AgentSessionStore) resource(namespace string) dynamic.ResourceInterface {
	return s.client.Dynamic().Resource(model.AgentSessionGVR()).Namespace(namespace)
}

// Save creates or updates the session CR (conflict-safe: one retry after
// re-reading the latest resourceVersion).
func (s *AgentSessionStore) Save(sess *model.AgentSession) error {
	if len(sess.Status.ContextSummary) > maxContextSummaryBytes {
		summary := sess.Status.ContextSummary[:maxContextSummaryBytes]
		// Never split a multi-byte UTF-8 rune at the cut point: the API
		// server (and JSON) require valid UTF-8 strings.
		for len(summary) > 0 && !utf8.Valid([]byte(summary)) {
			summary = summary[:len(summary)-1]
		}
		sess.Status.ContextSummary = summary
	}
	if sess.TypeMeta.Kind == "" {
		sess.TypeMeta = metav1.TypeMeta{APIVersion: "data.kubeai.alibabacloud.com/v1", Kind: "AgentSession"}
	}

	// The CRD declares a status subresource: spec and status must be written
	// separately — a plain Update() silently drops status changes on a real
	// API server (the fake client does not model this, so it is handled
	// explicitly here).
	for attempt := 0; attempt < 2; attempt++ {
		existing, err := s.Get(sess.Namespace, sess.Name)
		if err != nil {
			return err
		}
		if existing == nil {
			created, err := s.create(sess)
			if err != nil {
				if k8serrors.IsAlreadyExists(err) {
					continue // raced with another creator; retry as update
				}
				return err
			}
			return s.writeStatus(created, sess)
		}
		sess.ResourceVersion = existing.ResourceVersion
		updated, err := s.update(sess)
		if err != nil {
			if k8serrors.IsConflict(err) {
				continue // retry with the fresh resourceVersion
			}
			return err
		}
		return s.writeStatus(updated, sess)
	}
	return fmt.Errorf("save agentsession %s/%s: conflict after retry", sess.Namespace, sess.Name)
}

// writeStatus persists the status subresource on top of the freshest object
// state returned by the previous create/update.
func (s *AgentSessionStore) writeStatus(latest *unstructured.Unstructured, sess *model.AgentSession) error {
	b, err := json.Marshal(sess.Status)
	if err != nil {
		return err
	}
	var statusMap map[string]interface{}
	if err := json.Unmarshal(b, &statusMap); err != nil {
		return err
	}
	if err := unstructured.SetNestedField(latest.Object, statusMap, "status"); err != nil {
		return err
	}
	_, err = s.resource(sess.Namespace).UpdateStatus(context.TODO(), latest, metav1.UpdateOptions{})
	return err
}

// Get returns the session CR, or (nil, nil) when it does not exist.
func (s *AgentSessionStore) Get(namespace, name string) (*model.AgentSession, error) {
	raw, err := s.resource(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return fromUnstructured(raw)
}

// Delete removes the session CR; NotFound is treated as success (idempotent).
func (s *AgentSessionStore) Delete(namespace, name string) error {
	err := s.resource(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

// List returns all AgentSession CRs across namespaces. Callers filter by
// Spec.OwnerUser (the API layer never returns other users' sessions).
func (s *AgentSessionStore) List() ([]model.AgentSession, error) {
	raw, err := s.client.Dynamic().Resource(model.AgentSessionGVR()).
		Namespace(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var list model.AgentSessionList
	b, err := raw.MarshalJSON()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (s *AgentSessionStore) create(sess *model.AgentSession) (*unstructured.Unstructured, error) {
	u, err := toUnstructured(sess)
	if err != nil {
		return nil, err
	}
	return s.resource(sess.Namespace).Create(context.TODO(), u, metav1.CreateOptions{})
}

func (s *AgentSessionStore) update(sess *model.AgentSession) (*unstructured.Unstructured, error) {
	u, err := toUnstructured(sess)
	if err != nil {
		return nil, err
	}
	return s.resource(sess.Namespace).Update(context.TODO(), u, metav1.UpdateOptions{})
}

func toUnstructured(sess *model.AgentSession) (*unstructured.Unstructured, error) {
	b, err := json.Marshal(sess)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: m}, nil
}

func fromUnstructured(u *unstructured.Unstructured) (*model.AgentSession, error) {
	b, err := u.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var sess model.AgentSession
	if err := json.Unmarshal(b, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}
