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

package job

import (
	"context"

	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/cmd/options"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist/util"
	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	v1 "github.com/kubeflow/arena/pkg/operators/tf-operator/apis/tensorflow/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func init() {
	jobPersistCtrlMap[&v1.TFJob{}] = NewTFJobPersistController
}

func NewTFJobPersistController(mgr ctrl.Manager, handler *jobPersistHandler) PersistController {
	return &TFJobPersistController{
		client:  mgr.GetClient(),
		handler: handler,
	}
}

var _ reconcile.Reconciler = &TFJobPersistController{}

type TFJobPersistController struct {
	client  ctrlruntime.Client
	handler *jobPersistHandler
}

func (pc *TFJobPersistController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log.Info("starting reconciliation", "NamespacedName", req.NamespacedName)

	// Parse uid and object name from request.Name field.
	id, name, err := util.ParseIDName(req.Name)
	if err != nil {
		log.Error(err, "failed to parse request key")
		return ctrl.Result{}, err
	}

	tfJob := v1.TFJob{}
	err = pc.client.Get(context.Background(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      name,
	}, &tfJob)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("try to fetch tf job but it has been deleted.", "key", req.String())
			if err = pc.handler.Delete(req.Namespace, name, tfJob.Kind, id); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	newTfJob := arenaTFJobToKubeDLTFJob(&tfJob)
	// Persist tf job object into storage backend.
	if err = pc.handler.Write(newTfJob, newTfJob.Kind, newTfJob.Spec.TFReplicaSpecs, &newTfJob.Spec.RunPolicy, &newTfJob.Status); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

func (pc *TFJobPersistController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New("TFJobPersistController", mgr, controller.Options{
		Reconciler:              pc,
		MaxConcurrentReconciles: options.CtrlConfig.MaxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch events with event events-handler.
	if err = c.Watch(&source.Kind{Type: &v1.TFJob{}}, &enqueueForJob{}); err != nil {
		return err
	}
	return nil
}

func arenaTFJobToKubeDLTFJob(tfJob *v1.TFJob) *training.TFJob {
	newTF := training.TFJob{
		TypeMeta:   tfJob.TypeMeta,
		ObjectMeta: *tfJob.ObjectMeta.DeepCopy(),
	}

	cleanPodPolicy := apiv1.CleanPodPolicyRunning
	if tfJob.Spec.CleanPodPolicy != nil {
		cleanPodPolicy = apiv1.CleanPodPolicy(*tfJob.Spec.CleanPodPolicy)
	}
	newTF.Spec.RunPolicy = apiv1.RunPolicy{
		CleanPodPolicy:          &cleanPodPolicy,
		TTLSecondsAfterFinished: tfJob.Spec.TTLSecondsAfterFinished,
		ActiveDeadlineSeconds:   tfJob.Spec.ActiveDeadlineSeconds,
		BackoffLimit:            tfJob.Spec.BackoffLimit,
	}

	newTF.Spec.TFReplicaSpecs = make(map[apiv1.ReplicaType]*apiv1.ReplicaSpec)
	for rtype, rspec := range tfJob.Spec.TFReplicaSpecs {
		newTF.Spec.TFReplicaSpecs[apiv1.ReplicaType(rtype)] = &apiv1.ReplicaSpec{
			Replicas:      rspec.Replicas,
			Template:      *rspec.Template.DeepCopy(),
			RestartPolicy: apiv1.RestartPolicy(rspec.RestartPolicy),
		}
	}

	newTF.Status.StartTime = tfJob.Status.StartTime
	newTF.Status.CompletionTime = tfJob.Status.CompletionTime
	newTF.Status.LastReconcileTime = tfJob.Status.LastReconcileTime
	for _, cond := range tfJob.Status.Conditions {
		newTF.Status.Conditions = append(newTF.Status.Conditions, apiv1.JobCondition{
			Type:               apiv1.JobConditionType(cond.Type),
			Status:             cond.Status,
			Reason:             cond.Reason,
			Message:            cond.Message,
			LastUpdateTime:     cond.LastUpdateTime,
			LastTransitionTime: cond.LastTransitionTime,
		})
	}

	newTF.Status.ReplicaStatuses = make(map[apiv1.ReplicaType]*apiv1.ReplicaStatus)
	for rtype, rstatus := range tfJob.Status.ReplicaStatuses {
		newTF.Status.ReplicaStatuses[apiv1.ReplicaType(rtype)] = &apiv1.ReplicaStatus{
			Active:    rstatus.Active,
			Succeeded: rstatus.Succeeded,
			Failed:    rstatus.Failed,
		}
	}
	return &newTF
}
