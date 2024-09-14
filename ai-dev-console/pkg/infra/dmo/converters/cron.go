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

package converters

import (
	"encoding/json"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/apps/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"

	"k8s.io/klog"
)

func ConvertCronToDMOCron(cron *v1alpha1.Cron) *dmo.Cron {
	klog.V(5).Infof("[ConvertCronToDMOCron] cron: %s/%s", cron.Namespace, cron.Name)
	dmoCron := &dmo.Cron{
		Name:              cron.Name,
		Namespace:         cron.Namespace,
		UID:               string(cron.UID),
		Kind:              cron.Spec.CronTemplate.Kind,
		Schedule:          cron.Spec.Schedule,
		ConcurrencyPolicy: string(cron.Spec.ConcurrencyPolicy),
		GmtCreated:        cron.CreationTimestamp.Time,
	}

	if cron.Spec.Deadline != nil {
		dmoCron.Deadline = &cron.Spec.Deadline.Time
	}

	if cron.Spec.Suspend != nil {
		suspend := int8(0)
		if *cron.Spec.Suspend {
			suspend = 1
		}
		dmoCron.Suspend = &suspend
	}

	if cron.Spec.HistoryLimit != nil {
		dmoCron.HistoryLimit = cron.Spec.HistoryLimit
	}

	if cron.Status.LastScheduleTime != nil {
		dmoCron.LastScheduleTime = &cron.Status.LastScheduleTime.Time
	}

	labels := cron.GetLabels()
	if uid, ok := labels["arena.kubeflow.org/console-user"]; ok {
		dmoCron.User = &uid
	}

	dmoCron.Active = formatActiveList(cron)

	historyBytes, _ := json.Marshal(&cron.Status.History)
	dmoCron.History = string(historyBytes)
	return dmoCron
}

type simplifiedActiveEntry struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	UID       string `json:"uid"`
}

func formatActiveList(cron *v1alpha1.Cron) string {
	var actives []simplifiedActiveEntry
	for _, active := range cron.Status.Active {
		actives = append(actives, simplifiedActiveEntry{
			Name:      active.Name,
			Namespace: active.Namespace,
			UID:       string(active.UID),
		})
	}

	data, _ := json.Marshal(&actives)
	return string(data)
}
