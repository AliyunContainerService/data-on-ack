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

	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	v1 "github.com/kubeflow/arena/pkg/operators/pytorch-operator/apis/pytorch/v1"

	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/cmd/options"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist/util"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func init() {
	jobPersistCtrlMap[&v1.PyTorchJob{}] = NewPytorchJobPersistController
}

func NewPytorchJobPersistController(mgr ctrl.Manager, handler *jobPersistHandler) PersistController {
	return &PytorchJobPersistController{
		client:  mgr.GetClient(),
		handler: handler,
	}
}

var _ reconcile.Reconciler = &PytorchJobPersistController{}

type PytorchJobPersistController struct {
	client  ctrlruntime.Client
	handler *jobPersistHandler
}

func (pc *PytorchJobPersistController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log.Info("starting reconciliation", "NamespacedName", req.NamespacedName)

	// Parse uid and object name from request.Name field.
	id, name, err := util.ParseIDName(req.Name)
	if err != nil {
		log.Error(err, "failed to parse request key")
		return ctrl.Result{}, err
	}

	pytorchJob := v1.PyTorchJob{}
	err = pc.client.Get(context.Background(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      name,
	}, &pytorchJob)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("try to fetch pytorch job but it has been deleted.", "key", req.String())
			if err = pc.handler.Delete(req.Namespace, name, pytorchJob.Kind, id); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	newPyTorchJob := arenaPyTorchJobToKubeDLPyTorchJob(&pytorchJob)
	// Persist pytorch job object into storage backend.
	if err = pc.handler.Write(newPyTorchJob, newPyTorchJob.Kind, newPyTorchJob.Spec.PyTorchReplicaSpecs, &newPyTorchJob.Spec.RunPolicy, &newPyTorchJob.Status); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

func (pc *PytorchJobPersistController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New("PytorchJobPersistController", mgr, controller.Options{
		Reconciler:              pc,
		MaxConcurrentReconciles: options.CtrlConfig.MaxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch events with event events-handler.
	if err = c.Watch(&source.Kind{Type: &v1.PyTorchJob{}}, &enqueueForJob{}); err != nil {
		return err
	}
	return nil
}

func arenaPyTorchJobToKubeDLPyTorchJob(pyTorchJob *v1.PyTorchJob) *training.PyTorchJob {
	newPyTorch := training.PyTorchJob{
		TypeMeta:   pyTorchJob.TypeMeta,
		ObjectMeta: *pyTorchJob.ObjectMeta.DeepCopy(),
	}

	cleanPodPolicy := apiv1.CleanPodPolicyRunning
	if pyTorchJob.Spec.CleanPodPolicy != nil {
		cleanPodPolicy = apiv1.CleanPodPolicy(*pyTorchJob.Spec.CleanPodPolicy)
	}
	newPyTorch.Spec.RunPolicy = apiv1.RunPolicy{
		CleanPodPolicy:          &cleanPodPolicy,
		TTLSecondsAfterFinished: pyTorchJob.Spec.TTLSecondsAfterFinished,
		ActiveDeadlineSeconds:   pyTorchJob.Spec.ActiveDeadlineSeconds,
		BackoffLimit:            pyTorchJob.Spec.BackoffLimit,
	}

	newPyTorch.Spec.PyTorchReplicaSpecs = make(map[apiv1.ReplicaType]*apiv1.ReplicaSpec)
	for rtype, rspec := range pyTorchJob.Spec.PyTorchReplicaSpecs {
		newPyTorch.Spec.PyTorchReplicaSpecs[apiv1.ReplicaType(rtype)] = &apiv1.ReplicaSpec{
			Replicas:      rspec.Replicas,
			Template:      *rspec.Template.DeepCopy(),
			RestartPolicy: apiv1.RestartPolicy(rspec.RestartPolicy),
		}
	}

	newPyTorch.Status.StartTime = pyTorchJob.Status.StartTime
	newPyTorch.Status.CompletionTime = pyTorchJob.Status.CompletionTime
	newPyTorch.Status.LastReconcileTime = pyTorchJob.Status.LastReconcileTime
	for _, cond := range pyTorchJob.Status.Conditions {
		newPyTorch.Status.Conditions = append(newPyTorch.Status.Conditions, apiv1.JobCondition{
			Type:               apiv1.JobConditionType(cond.Type),
			Status:             cond.Status,
			Reason:             cond.Reason,
			Message:            cond.Message,
			LastUpdateTime:     cond.LastUpdateTime,
			LastTransitionTime: cond.LastTransitionTime,
		})
	}

	newPyTorch.Status.ReplicaStatuses = make(map[apiv1.ReplicaType]*apiv1.ReplicaStatus)
	for rtype, rstatus := range pyTorchJob.Status.ReplicaStatuses {
		newPyTorch.Status.ReplicaStatuses[apiv1.ReplicaType(rtype)] = &apiv1.ReplicaStatus{
			Active:    rstatus.Active,
			Succeeded: rstatus.Succeeded,
			Failed:    rstatus.Failed,
		}
	}
	return &newPyTorch
}
