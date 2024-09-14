/*
Copyright 2020 The Alibaba Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package converters

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

const (
	testRegion            = "test-region"
	testNamespace         = "kubedl-test"
	testMainContainerName = "tensorflow"
	testImage             = "kubedl/tf-mnist-with-summaries:1.0"
)

func TestConvertPodToDMOPod(t *testing.T) {
	type args struct {
		p      *corev1.Pod
		region string
	}
	tests := []struct {
		name    string
		args    args
		want    *dmo.Pod
		wantErr bool
	}{
		{
			name: "owner reference error",
			args: args{
				p: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							apiv1.AnnotationTenancyInfo: `{"tenant":"foo","user":"bar","idc":"test-idc","region":"test-region"}`,
						},
					},
				},
				region: testRegion,
			},
			want:    nil,
			wantErr: true,
		}, {
			name: "replica type error",
			args: args{
				p: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							apiv1.AnnotationTenancyInfo: `{"tenant":"foo","user":"bar","idc":"test-idc","region":"test-region"}`,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Controller: pointer.BoolPtr(true),
								UID:        "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
							},
						},
					},
				},
				region: testRegion,
			},
			want:    nil,
			wantErr: true,
		}, {
			name: "replica type in tf style",
			args: args{
				p: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "tfjob-0-test",
						Namespace:         testNamespace,
						UID:               "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
						ResourceVersion:   "3",
						CreationTimestamp: metav1.Time{Time: testTime("2019-02-10T12:27:00Z")},
						Labels:            map[string]string{"replica-type": "ps"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Controller: pointer.BoolPtr(true),
								UID:        "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  testMainContainerName,
								Image: testImage,
							},
						},
					},
					Status: corev1.PodStatus{},
				},
				region: testRegion,
			},
			want: &dmo.Pod{
				Name:        "tfjob-0-test",
				UID:         "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				Namespace:   testNamespace,
				EtcdVersion: "3",
				GmtCreated:  testTime("2019-02-10T12:27:00Z"),
				JobUID:      "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				ReplicaType: "ps",
				Image:       testImage,
				Status:      corev1.PodUnknown,
			},
			wantErr: false,
		}, {
			name: "success status Unknown",
			args: args{
				p: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "tfjob-0-test",
						Namespace:         testNamespace,
						UID:               "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
						ResourceVersion:   "3",
						CreationTimestamp: metav1.Time{Time: testTime("2019-02-10T12:27:00Z")},
						Labels:            map[string]string{"replica-type": "ps"},
						Annotations: map[string]string{
							apiv1.AnnotationTenancyInfo: `{"tenant":"foo","user":"bar","idc":"test-idc","region":"test-region"}`,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Controller: pointer.BoolPtr(true),
								UID:        "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  testMainContainerName,
								Image: testImage,
							},
						},
					},
					Status: corev1.PodStatus{},
				},
				region: testRegion,
			},
			want: &dmo.Pod{
				Name:        "tfjob-0-test",
				UID:         "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				Namespace:   testNamespace,
				EtcdVersion: "3",
				GmtCreated:  testTime("2019-02-10T12:27:00Z"),
				JobUID:      "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				ReplicaType: "ps",
				Image:       testImage,
				Status:      corev1.PodUnknown,
			},
			wantErr: false,
		}, {
			name: "success status Pending",
			args: args{
				p: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "tfjob-0-test",
						Namespace:         testNamespace,
						UID:               "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
						ResourceVersion:   "3",
						CreationTimestamp: metav1.Time{Time: testTime("2019-02-10T12:27:00Z")},
						Labels:            map[string]string{"replica-type": "ps"},
						Annotations: map[string]string{
							apiv1.AnnotationTenancyInfo: `{"tenant":"foo","user":"bar","idc":"test-idc","region":"test-region"}`,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Controller: pointer.BoolPtr(true),
								UID:        "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  testMainContainerName,
								Image: testImage,
							},
						},
					},
					Status: corev1.PodStatus{
						Phase:  corev1.PodPending,
						PodIP:  "127.0.0.1",
						HostIP: "192.168.1.1",
						ContainerStatuses: []corev1.ContainerStatus{{
							Name:  "",
							State: corev1.ContainerState{},
						}},
					},
				},
				region: testRegion,
			},
			want: &dmo.Pod{
				Name:        "tfjob-0-test",
				UID:         "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				Namespace:   testNamespace,
				EtcdVersion: "3",
				Image:       testImage,
				GmtCreated:  testTime("2019-02-10T12:27:00Z"),
				JobUID:      "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				ReplicaType: "ps",
				Status:      corev1.PodPending,
				PodJson:     "{\"metadata\":{\"name\":\"tfjob-0-test\",\"namespace\":\"kubedl-test\",\"uid\":\"6f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"resourceVersion\":\"3\",\"creationTimestamp\":\"2019-02-10T12:27:00Z\",\"labels\":{\"replica-type\":\"ps\"},\"annotations\":{\"kubedl.io/tenancy\":\"{\\\"tenant\\\":\\\"foo\\\",\\\"user\\\":\\\"bar\\\",\\\"idc\\\":\\\"test-idc\\\",\\\"region\\\":\\\"test-region\\\"}\"},\"ownerReferences\":[{\"apiVersion\":\"\",\"kind\":\"\",\"name\":\"\",\"uid\":\"7f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"controller\":true}]},\"spec\":{\"containers\":[{\"name\":\"tensorflow\",\"image\":\"kubedl/tf-mnist-with-summaries:1.0\",\"resources\":{}}]},\"status\":{}}",
				PodIP:       pointer.StringPtr("127.0.0.1"),
				HostIP:      pointer.StringPtr("192.168.1.1"),
			},
			wantErr: false,
		}, {
			name: "success status Running",
			args: args{
				p: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "tfjob-0-test",
						Namespace:         testNamespace,
						UID:               "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
						ResourceVersion:   "3",
						CreationTimestamp: metav1.Time{Time: testTime("2019-02-10T12:27:00Z")},
						Labels:            map[string]string{"replica-type": "ps"},
						Annotations: map[string]string{
							apiv1.AnnotationTenancyInfo: `{"tenant":"foo","user":"bar","idc":"test-idc","region":"test-region"}`,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Controller: pointer.BoolPtr(true),
								UID:        "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  testMainContainerName,
								Image: testImage,
							},
						},
					},
					Status: corev1.PodStatus{
						Phase:  corev1.PodRunning,
						PodIP:  "127.0.0.1",
						HostIP: "192.168.1.1",
						ContainerStatuses: []corev1.ContainerStatus{{
							Name: "",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.Time{Time: testTime("2019-02-10T12:28:00Z")},
								},
							},
						}},
					},
				},
				region: testRegion,
			},
			want: &dmo.Pod{
				Name:          "tfjob-0-test",
				UID:           "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				Namespace:     testNamespace,
				EtcdVersion:   "3",
				GmtCreated:    testTime("2019-02-10T12:27:00Z"),
				PodJson:       "{\"metadata\":{\"name\":\"tfjob-0-test\",\"namespace\":\"kubedl-test\",\"uid\":\"6f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"resourceVersion\":\"3\",\"creationTimestamp\":\"2019-02-10T12:27:00Z\",\"labels\":{\"replica-type\":\"ps\"},\"annotations\":{\"kubedl.io/tenancy\":\"{\\\"tenant\\\":\\\"foo\\\",\\\"user\\\":\\\"bar\\\",\\\"idc\\\":\\\"test-idc\\\",\\\"region\\\":\\\"test-region\\\"}\"},\"ownerReferences\":[{\"apiVersion\":\"\",\"kind\":\"\",\"name\":\"\",\"uid\":\"7f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"controller\":true}]},\"spec\":{\"containers\":[{\"name\":\"tensorflow\",\"image\":\"kubedl/tf-mnist-with-summaries:1.0\",\"resources\":{}}]},\"status\":{}}",
				JobUID:        "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				ReplicaType:   "ps",
				Image:         testImage,
				Status:        corev1.PodRunning,
				PodIP:         pointer.StringPtr("127.0.0.1"),
				HostIP:        pointer.StringPtr("192.168.1.1"),
				GmtPodRunning: testTimePtr("2019-02-10T12:28:00Z"),
			},
			wantErr: false,
		}, {
			name: "success status Succeeded",
			args: args{
				p: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "tfjob-0-test",
						Namespace:         testNamespace,
						UID:               "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
						ResourceVersion:   "3",
						CreationTimestamp: metav1.Time{Time: testTime("2019-02-10T12:27:00Z")},
						Labels:            map[string]string{"replica-type": "ps"},
						Annotations: map[string]string{
							apiv1.AnnotationTenancyInfo: `{"tenant":"foo","user":"bar","idc":"test-idc","region":"test-region"}`,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Controller: pointer.BoolPtr(true),
								UID:        "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  testMainContainerName,
								Image: testImage,
							},
						},
					},
					Status: corev1.PodStatus{
						Phase:  corev1.PodSucceeded,
						PodIP:  "127.0.0.1",
						HostIP: "192.168.1.1",
						ContainerStatuses: []corev1.ContainerStatus{{
							Name: testMainContainerName,
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.Time{Time: testTime("2019-02-10T12:26:00Z")},
								},
								Terminated: &corev1.ContainerStateTerminated{
									StartedAt:  metav1.Time{Time: testTime("2019-02-10T12:28:00Z")},
									FinishedAt: metav1.Time{Time: testTime("2019-02-11T12:28:00Z")},
								},
							},
						}},
					},
				},
				region: testRegion,
			},
			want: &dmo.Pod{
				Name:           "tfjob-0-test",
				UID:            "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				Namespace:      testNamespace,
				EtcdVersion:    "3",
				GmtCreated:     testTime("2019-02-10T12:27:00Z"),
				PodJson:        "{\"metadata\":{\"name\":\"tfjob-0-test\",\"namespace\":\"kubedl-test\",\"uid\":\"6f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"resourceVersion\":\"3\",\"creationTimestamp\":\"2019-02-10T12:27:00Z\",\"labels\":{\"replica-type\":\"ps\"},\"annotations\":{\"kubedl.io/tenancy\":\"{\\\"tenant\\\":\\\"foo\\\",\\\"user\\\":\\\"bar\\\",\\\"idc\\\":\\\"test-idc\\\",\\\"region\\\":\\\"test-region\\\"}\"},\"ownerReferences\":[{\"apiVersion\":\"\",\"kind\":\"\",\"name\":\"\",\"uid\":\"7f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"controller\":true}]},\"spec\":{\"containers\":[{\"name\":\"tensorflow\",\"image\":\"kubedl/tf-mnist-with-summaries:1.0\",\"resources\":{}}]},\"status\":{}}",
				JobUID:         "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				ReplicaType:    "ps",
				Image:          testImage,
				Status:         corev1.PodSucceeded,
				PodIP:          pointer.StringPtr("127.0.0.1"),
				HostIP:         pointer.StringPtr("192.168.1.1"),
				GmtPodRunning:  testTimePtr("2019-02-10T12:26:00Z"),
				GmtPodFinished: testTimePtr("2019-02-11T12:28:00Z"),
			},
			wantErr: false,
		}, {
			name: "success status Failed",
			args: args{
				p: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "tfjob-0-test",
						Namespace:         testNamespace,
						UID:               "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
						ResourceVersion:   "3",
						CreationTimestamp: metav1.Time{Time: testTime("2019-02-10T12:27:00Z")},
						Labels:            map[string]string{"replica-type": "ps"},
						Annotations: map[string]string{
							apiv1.AnnotationTenancyInfo: `{"tenant":"foo","user":"bar","idc":"test-idc","region":"test-region"}`,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Controller: pointer.BoolPtr(true),
								UID:        "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  testMainContainerName,
								Image: testImage,
							},
						},
					},
					Status: corev1.PodStatus{
						Phase:  corev1.PodFailed,
						PodIP:  "127.0.0.1",
						HostIP: "192.168.1.1",
						ContainerStatuses: []corev1.ContainerStatus{{
							Name: testMainContainerName,
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.Time{Time: testTime("2019-02-10T12:26:00Z")},
								},
								Terminated: &corev1.ContainerStateTerminated{
									StartedAt:  metav1.Time{Time: testTime("2019-02-10T12:28:00Z")},
									FinishedAt: metav1.Time{Time: testTime("2019-02-11T12:28:00Z")},
									ExitCode:   137,
									Reason:     "Reason07",
									Message:    "Message07",
								},
							},
						}},
					},
				},
				region: testRegion,
			},
			want: &dmo.Pod{
				Name:           "tfjob-0-test",
				UID:            "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				Namespace:      testNamespace,
				EtcdVersion:    "3",
				GmtCreated:     testTime("2019-02-10T12:27:00Z"),
				JobUID:         "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				ReplicaType:    "ps",
				Image:          testImage,
				PodJson:        "{\"metadata\":{\"name\":\"tfjob-0-test\",\"namespace\":\"kubedl-test\",\"uid\":\"6f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"resourceVersion\":\"3\",\"creationTimestamp\":\"2019-02-10T12:27:00Z\",\"labels\":{\"replica-type\":\"ps\"},\"annotations\":{\"kubedl.io/tenancy\":\"{\\\"tenant\\\":\\\"foo\\\",\\\"user\\\":\\\"bar\\\",\\\"idc\\\":\\\"test-idc\\\",\\\"region\\\":\\\"test-region\\\"}\"},\"ownerReferences\":[{\"apiVersion\":\"\",\"kind\":\"\",\"name\":\"\",\"uid\":\"7f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"controller\":true}]},\"spec\":{\"containers\":[{\"name\":\"tensorflow\",\"image\":\"kubedl/tf-mnist-with-summaries:1.0\",\"resources\":{}}]},\"status\":{}}",
				Status:         corev1.PodFailed,
				PodIP:          pointer.StringPtr("127.0.0.1"),
				HostIP:         pointer.StringPtr("192.168.1.1"),
				GmtPodRunning:  testTimePtr("2019-02-10T12:26:00Z"),
				GmtPodFinished: testTimePtr("2019-02-11T12:28:00Z"),
				Extended:       pointer.StringPtr("Reason: Reason07\nExitCode: 137\nMessage: Message07"),
			},
			wantErr: false,
		}, {
			name: "success without region",
			args: args{
				p: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "tfjob-0-test",
						Namespace:         testNamespace,
						UID:               "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
						ResourceVersion:   "3",
						CreationTimestamp: metav1.Time{Time: testTime("2019-02-10T12:27:00Z")},
						Labels:            map[string]string{"replica-type": "ps"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Controller: pointer.BoolPtr(true),
								UID:        "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  testMainContainerName,
								Image: testImage,
							},
						},
					},
					Status: corev1.PodStatus{
						Phase:  corev1.PodSucceeded,
						PodIP:  "127.0.0.1",
						HostIP: "192.168.1.1",
						ContainerStatuses: []corev1.ContainerStatus{{
							Name: testMainContainerName,
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.Time{Time: testTime("2019-02-10T12:26:00Z")},
								},
								Terminated: &corev1.ContainerStateTerminated{
									StartedAt:  metav1.Time{Time: testTime("2019-02-10T12:28:00Z")},
									FinishedAt: metav1.Time{Time: testTime("2019-02-11T12:28:00Z")},
								},
							},
						}},
					},
				},
			},
			want: &dmo.Pod{
				Name:           "tfjob-0-test",
				UID:            "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				Namespace:      testNamespace,
				EtcdVersion:    "3",
				GmtCreated:     testTime("2019-02-10T12:27:00Z"),
				JobUID:         "7f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				ReplicaType:    "ps",
				Image:          testImage,
				PodJson:        "{\"metadata\":{\"name\":\"tfjob-0-test\",\"namespace\":\"kubedl-test\",\"uid\":\"6f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"resourceVersion\":\"3\",\"creationTimestamp\":\"2019-02-10T12:27:00Z\",\"labels\":{\"replica-type\":\"ps\"},\"ownerReferences\":[{\"apiVersion\":\"\",\"kind\":\"\",\"name\":\"\",\"uid\":\"7f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"controller\":true}]},\"spec\":{\"containers\":[{\"name\":\"tensorflow\",\"image\":\"kubedl/tf-mnist-with-summaries:1.0\",\"resources\":{}}]},\"status\":{}}",
				Status:         corev1.PodSucceeded,
				PodIP:          pointer.StringPtr("127.0.0.1"),
				HostIP:         pointer.StringPtr("192.168.1.1"),
				GmtPodRunning:  testTimePtr("2019-02-10T12:26:00Z"),
				GmtPodFinished: testTimePtr("2019-02-11T12:28:00Z"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertPodToDMOPod(tt.args.p)
			if err != nil {
				if tt.wantErr {
					t.Logf("want err: %s", err)
				} else {
					t.Errorf("ConvertPodToDMO() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if tt.want != nil && tt.want.GmtPodFinished != nil && tt.want.GmtPodFinished.IsZero() && got.GmtPodFinished != nil && !got.GmtPodFinished.IsZero() {
				tt.want.GmtPodFinished = got.GmtPodFinished
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConvertPodToDMOPod(): got = %s, want %s", debugJson(got), debugJson(tt.want))
			}
		})
	}
}

// TestTime used for unit test only.
func testTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// TestTimePtr used for unit test only.
func testTimePtr(s string) *time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return &t
}

func debugJson(obj interface{}) string {
	b, _ := json.Marshal(obj)
	return string(b)
}
