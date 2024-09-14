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
	"fmt"
	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/cmd/options"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist/util"
	"github.com/kubeflow/arena/pkg/operators/mpi-operator/apis/kubeflow/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func init() {
	jobPersistCtrlMap[&v1alpha1.MPIJob{}] = NewMPIJobPersistController
}

func NewMPIJobPersistController(mgr ctrl.Manager, handler *jobPersistHandler) PersistController {
	return &MPIJobPersistController{
		client:  mgr.GetClient(),
		handler: handler,
	}
}

var _ reconcile.Reconciler = &MPIJobPersistController{}

type MPIJobPersistController struct {
	client  ctrlruntime.Client
	handler *jobPersistHandler
}

func (pc *MPIJobPersistController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log.Info("starting reconciliation", "NamespacedName", req.NamespacedName)

	// Parse uid and object name from request.Name field.
	id, name, err := util.ParseIDName(req.Name)
	if err != nil {
		log.Error(err, "failed to parse request key")
		return ctrl.Result{}, err
	}

	mpiJob := v1alpha1.MPIJob{}
	err = pc.client.Get(context.Background(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      name,
	}, &mpiJob)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("try to fetch mpi job but it has been deleted.", "key", req.String())
			if err = pc.handler.Delete(req.Namespace, name, mpiJob.Kind, id); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	newMPI := arenaMPIJobToKubeDLMPIJob(&mpiJob)

	// Persist mpi job object into storage backend.
	if err = pc.handler.Write(newMPI, newMPI.Kind, newMPI.Spec.MPIReplicaSpecs, &newMPI.Spec.RunPolicy, &newMPI.Status); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

func (pc *MPIJobPersistController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New("MPIJobPersistController", mgr, controller.Options{
		Reconciler:              pc,
		MaxConcurrentReconciles: options.CtrlConfig.MaxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch events with event events-handler.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.MPIJob{}}, &enqueueForJob{}); err != nil {
		return err
	}
	return nil
}

func arenaMPIJobToKubeDLMPIJob(mpiJob *v1alpha1.MPIJob) *training.MPIJob {
	newMPI := &training.MPIJob{
		TypeMeta:   mpiJob.TypeMeta,
		ObjectMeta: *mpiJob.ObjectMeta.DeepCopy(),
	}

	newMPI.Spec.LegacyV1Alpha1 = &training.LegacyV1Alpha1{
		GPUsPerNode:      mpiJob.Spec.GPUs,
		LauncherOnMaster: mpiJob.Spec.LauncherOnMaster,
		Replicas:         mpiJob.Spec.Replicas,
		Template:         mpiJob.Spec.Template,
	}
	newMPI.Spec.BackoffLimit = mpiJob.Spec.BackoffLimit

	_ = LegacyMPIJobToV1MPIJob(newMPI)

	return newMPI
}

func LegacyMPIJobToV1MPIJob(mpiJob *training.MPIJob) error {
	if mpiJob.Spec.MPIJobLegacySpec == nil {
		return nil
	}
	if mpiJob.Spec.MPIReplicaSpecs == nil {
		mpiJob.Spec.MPIReplicaSpecs = make(map[apiv1.ReplicaType]*apiv1.ReplicaSpec)
	}
	if mpiJob.Spec.MPIJobLegacySpec != nil && mpiJob.Spec.MPIJobLegacySpec.CleanPodPolicy != nil {
		mpiJob.Spec.RunPolicy.CleanPodPolicy = mpiJob.Spec.MPIJobLegacySpec.CleanPodPolicy
	}

	if v1alpha1 := mpiJob.Spec.MPIJobLegacySpec.LegacyV1Alpha1; v1alpha1 != nil {
		workerReplicas, unitsPerWorker, err := processingUnitsPerWorker(v1alpha1)

		if err != nil {
			return err
		}
		// The computed processing-units-per-worker will override SlotsPerWorker when
		// SlotsPerWorker is nil in v1alpha1 implementation.
		if mpiJob.Spec.SlotsPerWorker == nil && unitsPerWorker > 0 {
			mpiJob.Spec.SlotsPerWorker = &unitsPerWorker
		}
		// The computed worker will be converted to MPIReplicaSpecs struct.
		if spec := mpiJob.Spec.MPIReplicaSpecs[training.MPIReplicaTypeWorker]; (spec == nil || spec.Replicas == nil) &&
			workerReplicas > 0 {
			if spec == nil {
				spec = &apiv1.ReplicaSpec{}
			}
			spec.Replicas = &workerReplicas
			spec.Template = v1alpha1.Template
			mpiJob.Spec.MPIReplicaSpecs[training.MPIReplicaTypeWorker] = spec
		}

		if spec := mpiJob.Spec.MPIReplicaSpecs[training.MPIReplicaTypeLauncher]; spec == nil {
			mpiJob.Spec.MPIReplicaSpecs[training.MPIReplicaTypeLauncher] = &apiv1.ReplicaSpec{
				Replicas: pointer.Int32Ptr(1),
				Template: v1alpha1.Template,
			}
		}
	}

	if mpiJob.Spec.MPIJobLegacySpec.LegacyV1Alpha2 != nil {
		// The only differentiated point between versions is 'MPIDistribution', controller
		// handles this filed in effective position.
	}
	return nil
}

func processingUnitsPerWorker(v1alpha1 *training.LegacyV1Alpha1) (workerReplicas, unitsPerWorker int32, err error) {
	if v1alpha1.DeprecatedGPUs != nil && v1alpha1.ProcessingUnits != nil {
		return 0, 0, fmt.Errorf("cannot specify both GPUs and ProcessingUnits at the same time")
	}

	gpusPerNode := int32(1)
	if v1alpha1.GPUsPerNode != nil {
		gpusPerNode = *v1alpha1.GPUsPerNode
	}
	unitsPerNode := int32(1)
	if v1alpha1.ProcessingUnitsPerNode != nil {
		unitsPerNode = *v1alpha1.ProcessingUnitsPerNode
	}

	if v1alpha1.DeprecatedGPUs != nil || v1alpha1.ProcessingUnits != nil {
		totalUnits := int32(0)
		pusPerNode := int32(0)
		if v1alpha1.DeprecatedGPUs != nil {
			totalUnits = *v1alpha1.DeprecatedGPUs
			pusPerNode = gpusPerNode
		} else if v1alpha1.ProcessingUnits != nil {
			totalUnits = *v1alpha1.ProcessingUnits
			pusPerNode = unitsPerNode
		}

		if totalUnits < pusPerNode {
			workerReplicas = 1
			unitsPerWorker = totalUnits
		} else if totalUnits&pusPerNode == 0 {
			workerReplicas = totalUnits / pusPerNode
			unitsPerWorker = pusPerNode
		} else {
			return 0, 0, fmt.Errorf(
				"specified #ProcessingUnits(GPUs) must be a multiple of value per node(%d)", unitsPerNode)
		}
	} else if v1alpha1.Replicas != nil && len(v1alpha1.Template.Spec.Containers) > 0 {
		workerReplicas = *v1alpha1.Replicas
		container := &v1alpha1.Template.Spec.Containers[0]
		if container.Resources.Limits != nil && len(v1alpha1.ProcessingResourceType) > 0 {
			val := container.Resources.Limits[v1.ResourceName(v1alpha1.ProcessingResourceType)]
			processingUnits, _ := val.AsInt64()
			unitsPerWorker = int32(processingUnits)
		}
	}
	return
}
