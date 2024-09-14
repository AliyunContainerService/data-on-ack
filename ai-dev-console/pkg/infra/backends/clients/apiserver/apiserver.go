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

package apiserver

import (
	"context"
	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	clientmgr "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewAPIServerClientBackend() backends.ObjectClientBackend {
	return &apiServerBackend{client: clientmgr.GetCtrlClient()}
}

var (
	_        backends.ObjectClientBackend = &apiServerBackend{}
	allKinds []string
)

type apiServerBackend struct {
	client client.Client
}

func (a *apiServerBackend) DeleteEvaluateJob(ns, name string) error {
	return nil
}

func (a *apiServerBackend) SubmitEvaluateJob(evaluateJob *dmo.SubmitEvaluateJobInfo) error {
	return nil
}

func (a *apiServerBackend) Initialize() error {
	return nil
}

func (a *apiServerBackend) Close() error {
	return nil
}

func (a *apiServerBackend) Name() string {
	return "apiserver"
}

func (a *apiServerBackend) UserName(userName string) backends.ObjectClientBackend {
	return a
}

func initJobWithKind(kind string) (job runtime.Object) {
	switch kind {
	case training.TFJobKind:
		job = &training.TFJob{}
	case training.PyTorchJobKind:
		job = &training.PyTorchJob{}
	case training.XDLJobKind:
		job = &training.XDLJob{}
	case training.XGBoostJobKind:
		job = &training.XGBoostJob{}
	}
	return
}

func (a *apiServerBackend) SubmitJob(*dmo.SubmitJobInfo) error {
	return nil
}
func (a *apiServerBackend) StopJob(ns, name, jobID, kind string) error {
	job := initJobWithKind(kind)
	err := a.client.Get(context.Background(), types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return a.client.Delete(context.Background(), job)
}

func (a *apiServerBackend) SuspendCron(ns, name, cronID string) error {
	return nil
}

func (a *apiServerBackend) ResumeCron(ns, name, cronID string) error {
	return nil
}

func (a *apiServerBackend) StopCron(ns, name, cronID string) error {
	return nil
}
