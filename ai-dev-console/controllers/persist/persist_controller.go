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

package persist

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist/object/Notebook"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist/object/cron"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist/object/evaluate"
	"github.com/spf13/pflag"
	"os"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist/event"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist/object/job"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist/object/pod"

	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
)

func init() {
	pflag.StringVar(&region, "region", "", "region of kubedl deployed")
	pflag.StringVar(&eventStorage, "event-storage", "", "event storage backend plugin name, persist events into backend if it's specified")
	pflag.StringVar(&objectStorage, "object-storage", "", "object storage backend plugin name, persist jobs and pods into backend if it's specified")
}

var (
	region        string
	eventStorage  string
	objectStorage string
)

func SetupWithManager(mgr ctrl.Manager) error {
	klog.Infof("persist controller objectStorage: %s", objectStorage)
	if regionEnv, ok := os.LookupEnv("REGION"); ok {
		region = regionEnv
	}

	if eventStorage != "" {
		klog.Infof("event storage[%s] is set, init event-persist-controller", eventStorage)
		eventPersistController, err := event.NewEventPersistController(mgr, eventStorage, region)
		if err != nil {
			return err
		}
		if err = eventPersistController.SetupWithManager(mgr); err != nil {
			return err
		}
	}

	if objectStorage != "" {
		klog.Infof("object storage[%s] is set, init object-persist-controller", objectStorage)
		jobPersistController, err := job.NewJobPersistControllers(mgr, objectStorage, region)
		if err != nil {
			return err
		}
		if err = jobPersistController.SetupWithManager(mgr); err != nil {
			return err
		}
		podPersistController, err := pod.NewPodPersistController(mgr, objectStorage, region)
		if err != nil {
			return err
		}
		if err = podPersistController.SetupWithManager(mgr); err != nil {
			return err
		}
		cronPersistController, err := cron.NewCronPersistController(mgr, objectStorage, region)
		if err != nil {
			return err
		}
		if err = cronPersistController.SetupWithManager(mgr); err != nil {
			return err
		}
		evaluateJobPersistController, err := evaluate.NewEvaluateJobPersistController(mgr, objectStorage, region)
		if err != nil {
			return err
		}
		if err = evaluateJobPersistController.SetupWithManager(mgr); err != nil {
			return err
		}
		notebookPersistController, err := Notebook.NewNotebookPersistController(mgr, objectStorage, region)
		if err != nil {
			return err
		}
		if err = notebookPersistController.SetupWithManager(mgr); err != nil {
			return err
		}
	}
	return nil
}
