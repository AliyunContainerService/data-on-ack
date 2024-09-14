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

package cron

import (
	"context"
	stderrors "errors"
	"fmt"

	appsv1alpha1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/apps/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/cmd/options"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist/util"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/registry"

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
	controllerName = "CronPersistController"
)

var log = logf.Log.WithName("cron-persist-controller")

func NewCronPersistController(mgr ctrl.Manager, cronStorage string, region string) (*CronPersistController, error) {
	if cronStorage == "" {
		return nil, stderrors.New("empty cron storage backend name")
	}

	// Get pod storage backend from backends registry.
	cronBackend := registry.GetObjectBackend(cronStorage)
	if cronBackend == nil {
		return nil, fmt.Errorf("cron storage backend [%s] has not registered", cronStorage)
	}

	// Initialize cron storage backend before pod-persist-controller created.
	if err := cronBackend.Initialize(); err != nil {
		return nil, err
	}

	return &CronPersistController{
		region:      region,
		client:      mgr.GetClient(),
		cronBackend: cronBackend,
	}, nil
}

var _ reconcile.Reconciler = &CronPersistController{}

type CronPersistController struct {
	region      string
	client      ctrlruntime.Client
	cronBackend backends.ObjectStorageBackend
}

func (pc *CronPersistController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log.Info("starting reconciliation", "NamespacedName", req.NamespacedName)

	// Parse uid and object name from request.Name field.
	id, name, err := util.ParseIDName(req.Name)
	if err != nil {
		log.Error(err, "failed to parse request key")
		return ctrl.Result{}, err
	}

	cron := appsv1alpha1.Cron{}
	err = pc.client.Get(context.Background(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      name,
	}, &cron)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("try to fetch cron but it has been deleted.", "key", req.String())

			if err = pc.cronBackend.DeleteCron(req.Namespace, name, id); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Persist cron object into storage backend.
	if err = pc.cronBackend.WriteCron(&cron); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

func (pc *CronPersistController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New(controllerName, mgr, controller.Options{
		Reconciler:              pc,
		MaxConcurrentReconciles: options.CtrlConfig.MaxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch events with event events-handler.
	if err = c.Watch(&source.Kind{Type: &appsv1alpha1.Cron{}}, &enqueueForCron{}); err != nil {
		return err
	}
	return nil
}
