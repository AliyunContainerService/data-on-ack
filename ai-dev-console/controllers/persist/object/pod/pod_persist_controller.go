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

package pod

import (
	"context"
	stderrors "errors"
	"fmt"
	"math"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/cmd/options"
	persistutil "github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist/util"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/registry"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "PodPersistController"
)

var log = logf.Log.WithName("pod-persist-controller")

func NewPodPersistController(mgr ctrl.Manager, podStorage string, region string) (*PodPersistController, error) {
	if podStorage == "" {
		return nil, stderrors.New("empty pod storage backend name")
	}

	// Get pod storage backend from backends registry.
	podBackend := registry.GetObjectBackend(podStorage)
	if podBackend == nil {
		return nil, fmt.Errorf("pod storage backend [%s] has not registered", podStorage)
	}

	// Initialize pod storage backend before pod-persist-controller created.
	if err := podBackend.Initialize(); err != nil {
		return nil, err
	}

	return &PodPersistController{
		region:     region,
		client:     mgr.GetClient(),
		podBackend: podBackend,
	}, nil
}

var _ reconcile.Reconciler = &PodPersistController{}

type PodPersistController struct {
	region     string
	client     ctrlruntime.Client
	podBackend backends.ObjectStorageBackend
}

func (pc *PodPersistController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log.Info("starting reconciliation", "NamespacedName", req.NamespacedName)
	// Parse uid and object name from request.Name field.
	id, name, err := persistutil.ParseIDName(req.Name)
	if err != nil {
		log.Error(err, "failed to parse request key")
		return ctrl.Result{}, err
	}

	pod := &corev1.Pod{}
	err = pc.client.Get(context.Background(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      name,
	}, pod)

	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("try to fetch pod but it has been deleted.", "key", req.String())
			if err = pc.podBackend.UpdatePodRecordStopped(req.Namespace, name, id); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if pod.DeletionTimestamp != nil {
		// Deletion timestamp has been set for positively cleaning up.
		log.Info("pod has been deleted and deletion timestamp set", "key", req.String())
		if err = pc.podBackend.UpdatePodRecordStopped(req.Namespace, name, id); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err = pc.podBackend.WritePod(pod); err != nil {
		log.Error(err, "error when persist pod object to storage backend", "pod", req.String())
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (pc *PodPersistController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New(controllerName, mgr, controller.Options{
		Reconciler:              pc,
		MaxConcurrentReconciles: int(math.Max(10, float64(options.CtrlConfig.MaxConcurrentReconciles))),
	})
	if err != nil {
		return err
	}

	// Watch events with pod events-handler.
	if err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &enqueueForPod{}); err != nil {
		return err
	}
	return nil
}
