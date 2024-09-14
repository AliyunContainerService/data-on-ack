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
	"reflect"
	"testing"

	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/util"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestConvertJobToDMOJob(t *testing.T) {
	type args struct {
		job    metav1.Object
		region string
	}
	tests := []struct {
		name    string
		args    args
		want    *dmo.Job
		wantErr bool
	}{
		{
			name: "tfjob with created status",
			args: args{
				job: &training.TFJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "tfjob-test",
						Namespace:         testNamespace,
						UID:               "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
						ResourceVersion:   "3",
						CreationTimestamp: metav1.Time{Time: testTime("2019-02-10T12:27:00Z")},
					},
					Status: apiv1.JobStatus{
						StartTime: &metav1.Time{Time: testTime("2019-02-11T12:27:00Z")},
					},
					Spec: training.TFJobSpec{
						TFReplicaSpecs: map[apiv1.ReplicaType]*apiv1.ReplicaSpec{
							"Worker": {
								Template: v1.PodTemplateSpec{
									Spec: v1.PodSpec{
										Containers: []v1.Container{
											{
												Image: testImage,
												Resources: v1.ResourceRequirements{Requests: v1.ResourceList{
													"cpu":    resource.MustParse("1"),
													"memory": resource.MustParse("1Gi"),
												},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				region: testRegion,
			},
			want: &dmo.Job{
				Name:            "tfjob-test",
				UID:             "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				Kind:            training.TFJobKind,
				Status:          apiv1.JobCreated,
				Namespace:       testNamespace,
				RegionID:        pointer.StringPtr(testRegion),
				Tenant:          pointer.StringPtr(""),
				User:            pointer.StringPtr(""),
				IsDeleted:       util.IntPtr(0),
				IsInK8s:         1,
				JobJson:         "{\"metadata\":{\"name\":\"tfjob-test\",\"namespace\":\"kubedl-test\",\"uid\":\"6f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"resourceVersion\":\"3\",\"creationTimestamp\":\"2019-02-10T12:27:00Z\"},\"spec\":{\"tfReplicaSpecs\":{\"Worker\":{\"template\":{\"metadata\":{\"creationTimestamp\":null},\"spec\":{\"containers\":[{\"name\":\"\",\"image\":\"kubedl/tf-mnist-with-summaries:1.0\",\"resources\":{\"requests\":{\"cpu\":\"1\",\"memory\":\"1Gi\"}}}]}}}}},\"status\":{\"conditions\":null,\"replicaStatuses\":null,\"startTime\":\"2019-02-11T12:27:00Z\"}}",
				EtcdVersion:     "3",
				GmtJobSubmitted: testTime("2019-02-10T12:27:00Z"),
				Resources:       `{"Worker":{"resources":{"requests":{"cpu":"1","memory":"1Gi"}},"replicas":1}}`,
				JobConfig:       "{}",
			},
		}, {
			name: "tfjob with region",
			args: args{
				job: &training.TFJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "tfjob-test",
						Namespace:         testNamespace,
						UID:               "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
						ResourceVersion:   "3",
						CreationTimestamp: metav1.Time{Time: testTime("2019-02-10T12:27:00Z")},
						Annotations: map[string]string{
							apiv1.AnnotationTenancyInfo: `{"tenant":"foo","user":"bar","idc":"test-idc","region":"test-region"}`,
						},
					},
					Status: apiv1.JobStatus{
						CompletionTime: &metav1.Time{Time: testTime("2019-02-11T12:27:00Z")},
						Conditions:     []apiv1.JobCondition{{Type: apiv1.JobSucceeded}},
					},
					Spec: training.TFJobSpec{
						TFReplicaSpecs: map[apiv1.ReplicaType]*apiv1.ReplicaSpec{
							"Worker": {
								Replicas: pointer.Int32Ptr(1),
								Template: v1.PodTemplateSpec{
									Spec: v1.PodSpec{
										Containers: []v1.Container{
											{
												Name:  testMainContainerName,
												Image: testImage,
												Resources: v1.ResourceRequirements{
													Requests: v1.ResourceList{"cpu": resource.MustParse("1"), "memory": resource.MustParse("1Gi")},
												},
											},
											{
												Name:  "sidecar",
												Image: testImage,
												Resources: v1.ResourceRequirements{
													Requests: v1.ResourceList{"cpu": resource.MustParse("1"), "memory": resource.MustParse("1Gi")},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &dmo.Job{
				Name:            "tfjob-test",
				UID:             "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				Kind:            training.TFJobKind,
				Status:          apiv1.JobSucceeded,
				Namespace:       testNamespace,
				RegionID:        pointer.StringPtr(testRegion),
				Tenant:          pointer.StringPtr("foo"),
				Group:           pointer.StringPtr(""),
				User:            pointer.StringPtr("bar"),
				IsDeleted:       util.IntPtr(0),
				IsInK8s:         1,
				JobJson:         "{\"metadata\":{\"name\":\"tfjob-test\",\"namespace\":\"kubedl-test\",\"uid\":\"6f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"resourceVersion\":\"3\",\"creationTimestamp\":\"2019-02-10T12:27:00Z\",\"annotations\":{\"kubedl.io/tenancy\":\"{\\\"tenant\\\":\\\"foo\\\",\\\"user\\\":\\\"bar\\\",\\\"idc\\\":\\\"test-idc\\\",\\\"region\\\":\\\"test-region\\\"}\"}},\"spec\":{\"tfReplicaSpecs\":{\"Worker\":{\"replicas\":1,\"template\":{\"metadata\":{\"creationTimestamp\":null},\"spec\":{\"containers\":[{\"name\":\"tensorflow\",\"image\":\"kubedl/tf-mnist-with-summaries:1.0\",\"resources\":{\"requests\":{\"cpu\":\"1\",\"memory\":\"1Gi\"}}},{\"name\":\"sidecar\",\"image\":\"kubedl/tf-mnist-with-summaries:1.0\",\"resources\":{\"requests\":{\"cpu\":\"1\",\"memory\":\"1Gi\"}}}]}}}}},\"status\":{\"conditions\":[{\"type\":\"Succeeded\",\"status\":\"\",\"lastUpdateTime\":null,\"lastTransitionTime\":null}],\"replicaStatuses\":null,\"completionTime\":\"2019-02-11T12:27:00Z\"}}",
				EtcdVersion:     "3",
				Reason:          pointer.StringPtr(""),
				ReasonCode:      pointer.StringPtr(""),
				GmtJobSubmitted: testTime("2019-02-10T12:27:00Z"),
				GmtJobFinished:  testTimePtr("2019-02-11T12:27:00Z"),
				Resources:       `{"Worker":{"resources":{"requests":{"cpu":"2","memory":"2Gi"}},"replicas":1}}`,
				JobConfig:       "{}",
			},
		}, {
			name: "xdljob with status succeed",
			args: args{
				job: &training.XDLJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "xdljob-test",
						Namespace:         testNamespace,
						UID:               "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
						ResourceVersion:   "3",
						CreationTimestamp: metav1.Time{Time: testTime("2019-02-10T12:27:00Z")},
					},
					Status: apiv1.JobStatus{
						CompletionTime: &metav1.Time{Time: testTime("2019-02-11T12:27:00Z")},
						Conditions:     []apiv1.JobCondition{{Type: apiv1.JobSucceeded}},
					},
					Spec: training.XDLJobSpec{
						XDLReplicaSpecs: map[apiv1.ReplicaType]*apiv1.ReplicaSpec{
							"Master": {
								Replicas: pointer.Int32Ptr(1),
								Template: v1.PodTemplateSpec{
									Spec: v1.PodSpec{
										Containers: []v1.Container{
											{
												Name:  testMainContainerName,
												Image: testImage,
												Resources: v1.ResourceRequirements{
													Requests: v1.ResourceList{"cpu": resource.MustParse("1"), "memory": resource.MustParse("1Gi")},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &dmo.Job{
				Name:            "xdljob-test",
				UID:             "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				Kind:            training.XDLJobKind,
				Status:          apiv1.JobSucceeded,
				Namespace:       testNamespace,
				Tenant:          pointer.StringPtr(""),
				User:            pointer.StringPtr(""),
				IsDeleted:       util.IntPtr(0),
				IsInK8s:         1,
				JobJson:         "{\"metadata\":{\"name\":\"xdljob-test\",\"namespace\":\"kubedl-test\",\"uid\":\"6f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"resourceVersion\":\"3\",\"creationTimestamp\":\"2019-02-10T12:27:00Z\"},\"spec\":{\"xdlReplicaSpecs\":{\"Master\":{\"replicas\":1,\"template\":{\"metadata\":{\"creationTimestamp\":null},\"spec\":{\"containers\":[{\"name\":\"tensorflow\",\"image\":\"kubedl/tf-mnist-with-summaries:1.0\",\"resources\":{\"requests\":{\"cpu\":\"1\",\"memory\":\"1Gi\"}}}]}}}}},\"status\":{\"conditions\":[{\"type\":\"Succeeded\",\"status\":\"\",\"lastUpdateTime\":null,\"lastTransitionTime\":null}],\"replicaStatuses\":null,\"completionTime\":\"2019-02-11T12:27:00Z\"}}",
				EtcdVersion:     "3",
				Reason:          pointer.StringPtr(""),
				ReasonCode:      pointer.StringPtr(""),
				GmtJobSubmitted: testTime("2019-02-10T12:27:00Z"),
				GmtJobFinished:  testTimePtr("2019-02-11T12:27:00Z"),
				Resources:       `{"Master":{"resources":{"requests":{"cpu":"1","memory":"1Gi"}},"replicas":1}}`,
				JobConfig:       "{}",
			},
		}, {
			name: "pytorchjob with succeed status",
			args: args{
				job: &training.PyTorchJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "pytorchjob-test",
						Namespace:         testNamespace,
						UID:               "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
						ResourceVersion:   "3",
						CreationTimestamp: metav1.Time{Time: testTime("2019-02-10T12:27:00Z")},
					},
					Status: apiv1.JobStatus{
						CompletionTime: &metav1.Time{Time: testTime("2019-02-11T12:27:00Z")},
						Conditions:     []apiv1.JobCondition{{Type: apiv1.JobSucceeded}},
					},
					Spec: training.PyTorchJobSpec{
						PyTorchReplicaSpecs: map[apiv1.ReplicaType]*apiv1.ReplicaSpec{
							"Worker": {
								Replicas: pointer.Int32Ptr(1),
								Template: v1.PodTemplateSpec{
									Spec: v1.PodSpec{
										Containers: []v1.Container{
											{
												Name:  testMainContainerName,
												Image: testImage,
												Resources: v1.ResourceRequirements{
													Requests: v1.ResourceList{"cpu": resource.MustParse("1"), "memory": resource.MustParse("1Gi")},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &dmo.Job{
				Name:            "pytorchjob-test",
				UID:             "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				Kind:            training.PyTorchJobKind,
				Status:          apiv1.JobSucceeded,
				Namespace:       testNamespace,
				Tenant:          pointer.StringPtr(""),
				User:            pointer.StringPtr(""),
				IsDeleted:       util.IntPtr(0),
				IsInK8s:         1,
				JobJson:         "{\"metadata\":{\"name\":\"pytorchjob-test\",\"namespace\":\"kubedl-test\",\"uid\":\"6f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"resourceVersion\":\"3\",\"creationTimestamp\":\"2019-02-10T12:27:00Z\"},\"spec\":{\"pytorchReplicaSpecs\":{\"Worker\":{\"replicas\":1,\"template\":{\"metadata\":{\"creationTimestamp\":null},\"spec\":{\"containers\":[{\"name\":\"tensorflow\",\"image\":\"kubedl/tf-mnist-with-summaries:1.0\",\"resources\":{\"requests\":{\"cpu\":\"1\",\"memory\":\"1Gi\"}}}]}}}}},\"status\":{\"conditions\":[{\"type\":\"Succeeded\",\"status\":\"\",\"lastUpdateTime\":null,\"lastTransitionTime\":null}],\"replicaStatuses\":null,\"completionTime\":\"2019-02-11T12:27:00Z\"}}",
				EtcdVersion:     "3",
				Reason:          pointer.StringPtr(""),
				ReasonCode:      pointer.StringPtr(""),
				GmtJobSubmitted: testTime("2019-02-10T12:27:00Z"),
				GmtJobFinished:  testTimePtr("2019-02-11T12:27:00Z"),
				Resources:       `{"Worker":{"resources":{"requests":{"cpu":"1","memory":"1Gi"}},"replicas":1}}`,
				JobConfig:       "{}",
			},
		}, {
			name: "xgboostjob with region",
			args: args{
				job: &training.XGBoostJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "xgboostjob-test",
						Namespace:         testNamespace,
						UID:               "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
						ResourceVersion:   "3",
						CreationTimestamp: metav1.Time{Time: testTime("2019-02-10T12:27:00Z")},
					},
					Status: training.XGBoostJobStatus{
						JobStatus: apiv1.JobStatus{
							CompletionTime: &metav1.Time{Time: testTime("2019-02-11T12:27:00Z")},
							Conditions:     []apiv1.JobCondition{{Type: apiv1.JobSucceeded}},
						},
					},
					Spec: training.XGBoostJobSpec{
						XGBReplicaSpecs: map[apiv1.ReplicaType]*apiv1.ReplicaSpec{
							"Worker": {
								Replicas: pointer.Int32Ptr(1),
								Template: v1.PodTemplateSpec{
									Spec: v1.PodSpec{
										Containers: []v1.Container{
											{
												Name:  testMainContainerName,
												Image: testImage,
												Resources: v1.ResourceRequirements{
													Requests: v1.ResourceList{"cpu": resource.MustParse("1"), "memory": resource.MustParse("1Gi")},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &dmo.Job{
				Name:            "xgboostjob-test",
				UID:             "6f06d2fd-22c6-11e9-96bb-0242ac1d5327",
				Kind:            training.XGBoostJobKind,
				Status:          apiv1.JobSucceeded,
				Namespace:       testNamespace,
				Tenant:          pointer.StringPtr(""),
				User:            pointer.StringPtr(""),
				IsDeleted:       util.IntPtr(0),
				IsInK8s:         1,
				JobJson:         "{\"metadata\":{\"name\":\"xgboostjob-test\",\"namespace\":\"kubedl-test\",\"uid\":\"6f06d2fd-22c6-11e9-96bb-0242ac1d5327\",\"resourceVersion\":\"3\",\"creationTimestamp\":\"2019-02-10T12:27:00Z\"},\"spec\":{\"RunPolicy\":{},\"xgbReplicaSpecs\":{\"Worker\":{\"replicas\":1,\"template\":{\"metadata\":{\"creationTimestamp\":null},\"spec\":{\"containers\":[{\"name\":\"tensorflow\",\"image\":\"kubedl/tf-mnist-with-summaries:1.0\",\"resources\":{\"requests\":{\"cpu\":\"1\",\"memory\":\"1Gi\"}}}]}}}}},\"status\":{\"conditions\":[{\"type\":\"Succeeded\",\"status\":\"\",\"lastUpdateTime\":null,\"lastTransitionTime\":null}],\"replicaStatuses\":null,\"completionTime\":\"2019-02-11T12:27:00Z\"}}",
				EtcdVersion:     "3",
				Reason:          pointer.StringPtr(""),
				ReasonCode:      pointer.StringPtr(""),
				GmtJobSubmitted: testTime("2019-02-10T12:27:00Z"),
				GmtJobFinished:  testTimePtr("2019-02-11T12:27:00Z"),
				Resources:       `{"Worker":{"resources":{"requests":{"cpu":"1","memory":"1Gi"}},"replicas":1}}`,
				JobConfig:       "{}",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, spec, status, err := ExtractTypedJobInfos(tt.args.job)
			if err != nil {
				t.Errorf("failed to extract job info, err: %v", err)
				return
			}
			got, err := ConvertJobToDMOJob(tt.args.job, kind, spec, &status, tt.args.region, false)
			if err != nil {
				t.Errorf("failed to convert to dmo job, err: %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConvertPodToDMO(): got = %v, want %v", debugJson(got), debugJson(tt.want))
			}
		})
	}
}
