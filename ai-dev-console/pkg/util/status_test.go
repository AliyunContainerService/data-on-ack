/*
Copyright 2021 The Alibaba Authors.

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

package util

import (
	"testing"

	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestIsSucceeded(t *testing.T) {
	jobStatus := apiv1.JobStatus{
		Conditions: []apiv1.JobCondition{
			{
				Type:   apiv1.JobSucceeded,
				Status: corev1.ConditionTrue,
			},
		},
	}
	assert.True(t, IsSucceeded(jobStatus))
}

func TestIsFailed(t *testing.T) {
	jobStatus := apiv1.JobStatus{
		Conditions: []apiv1.JobCondition{
			{
				Type:   apiv1.JobFailed,
				Status: corev1.ConditionTrue,
			},
		},
	}
	assert.True(t, IsFailed(jobStatus))
}

func TestUpdateJobConditions(t *testing.T) {
	jobStatus := apiv1.JobStatus{}
	conditionType := apiv1.JobCreated
	reason := "Job Created"
	message := "Job Created"

	err := UpdateJobConditions(&jobStatus, conditionType, reason, message)
	if assert.NoError(t, err) {
		// Check JobCreated condition is appended
		conditionInStatus := jobStatus.Conditions[0]
		assert.Equal(t, conditionInStatus.Type, conditionType)
		assert.Equal(t, conditionInStatus.Reason, reason)
		assert.Equal(t, conditionInStatus.Message, message)
	}

	conditionType = apiv1.JobRunning
	reason = "Job Running"
	message = "Job Running"
	err = UpdateJobConditions(&jobStatus, conditionType, reason, message)
	if assert.NoError(t, err) {
		// Check JobRunning condition is appended
		conditionInStatus := jobStatus.Conditions[1]
		assert.Equal(t, conditionInStatus.Type, conditionType)
		assert.Equal(t, conditionInStatus.Reason, reason)
		assert.Equal(t, conditionInStatus.Message, message)
	}

	conditionType = apiv1.JobRestarting
	reason = "Job Restarting"
	message = "Job Restarting"
	err = UpdateJobConditions(&jobStatus, conditionType, reason, message)
	if assert.NoError(t, err) {
		// Check JobRunning condition is filtered out and JobRestarting state is appended
		conditionInStatus := jobStatus.Conditions[1]
		assert.Equal(t, conditionInStatus.Type, conditionType)
		assert.Equal(t, conditionInStatus.Reason, reason)
		assert.Equal(t, conditionInStatus.Message, message)
	}

	conditionType = apiv1.JobRunning
	reason = "Job Running"
	message = "Job Running"
	err = UpdateJobConditions(&jobStatus, conditionType, reason, message)
	if assert.NoError(t, err) {
		// Again, Check JobRestarting condition is filtered and JobRestarting is appended
		conditionInStatus := jobStatus.Conditions[1]
		assert.Equal(t, conditionInStatus.Type, conditionType)
		assert.Equal(t, conditionInStatus.Reason, reason)
		assert.Equal(t, conditionInStatus.Message, message)
	}

	conditionType = apiv1.JobFailed
	reason = "Job Failed"
	message = "Job Failed"
	err = UpdateJobConditions(&jobStatus, conditionType, reason, message)
	if assert.NoError(t, err) {
		// Check JobRunning condition is set to false
		jobRunningCondition := jobStatus.Conditions[1]
		assert.Equal(t, jobRunningCondition.Type, apiv1.JobRunning)
		assert.Equal(t, jobRunningCondition.Status, corev1.ConditionFalse)
		// Check JobFailed state is appended
		conditionInStatus := jobStatus.Conditions[2]
		assert.Equal(t, conditionInStatus.Type, conditionType)
		assert.Equal(t, conditionInStatus.Reason, reason)
		assert.Equal(t, conditionInStatus.Message, message)
	}
}
