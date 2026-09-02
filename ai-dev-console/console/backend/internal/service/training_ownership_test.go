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
	"testing"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestTrainingService(pods ...*corev1.Pod) *TrainingService {
	fc := fake.NewSimpleClientset()
	for _, p := range pods {
		_, _ = fc.CoreV1().Pods(p.Namespace).Create(testCtx(), p, metav1.CreateOptions{})
	}
	return NewTrainingService(k8s.NewClientForTesting(fc, nil), nil)
}

func pod(ns, name string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, Labels: labels}}
}

func TestEnsurePodBelongsToJob(t *testing.T) {
	svc := newTestTrainingService(
		pod("ns1", "myjob-worker-0", map[string]string{"job-name": "myjob"}),
		pod("ns1", "rayjob-raycluster-abc-ray-worker", map[string]string{"ray.io/cluster": "rayjob-raycluster-abc"}),
		pod("ns1", "victim-daemon", map[string]string{"app": "victim"}),
		pod("ns1", "myjob-evil", nil), // name prefix match but NO labels
	)

	cases := []struct {
		name    string
		job     string
		podName string
		wantErr bool
	}{
		{"label match", "myjob", "myjob-worker-0", false},
		{"ray cluster match", "rayjob", "rayjob-raycluster-abc-ray-worker", false},
		{"unrelated pod", "myjob", "victim-daemon", true},
		{"prefix without label must fail (B1)", "myjob", "myjob-evil", true},
		{"unknown pod", "myjob", "ghost-0", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := svc.EnsurePodBelongsToJob("ns1", c.job, c.podName)
			if (err != nil) != c.wantErr {
				t.Fatalf("EnsurePodBelongsToJob(%q,%q) err=%v wantErr=%v", c.job, c.podName, err, c.wantErr)
			}
		})
	}
}
