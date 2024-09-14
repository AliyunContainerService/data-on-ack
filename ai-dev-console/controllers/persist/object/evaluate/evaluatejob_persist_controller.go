/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
*limitations under the License.
 */

package evaluate

import (
	"context"
	stderrors "errors"
	"fmt"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/cmd/options"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/controllers/persist/util"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/registry"
	batch "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "EvaluateJobPersistController"
)

var log = logf.Log.WithName("evaluateJob-persist-controller")

func NewEvaluateJobPersistController(mgr ctrl.Manager, evaluateStorage string, region string) (*EvaluateJobPersistController, error) {
	if evaluateStorage == "" {
		return nil, stderrors.New("empty evaluateJob storage backend name")
	}

	// Get pod storage backend from backends registry.
	evaluateBackend := registry.GetObjectBackend(evaluateStorage)
	if evaluateBackend == nil {
		return nil, fmt.Errorf("evaluateJob storage backend [%s] has not registered", evaluateStorage)
	}

	// Initialize evaluate storage backend before pod-persist-controller created.
	if err := evaluateBackend.Initialize(); err != nil {
		return nil, err
	}

	return &EvaluateJobPersistController{
		region:          region,
		client:          mgr.GetClient(),
		evaluateBackend: evaluateBackend,
	}, nil
}

var _ reconcile.Reconciler = &EvaluateJobPersistController{}

type EvaluateJobPersistController struct {
	region          string
	client          ctrlruntime.Client
	evaluateBackend backends.ObjectStorageBackend
}

func (pc *EvaluateJobPersistController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log.Info("starting reconciliation", "NamespacedName", req.NamespacedName)
	// Parse uid and object name from request.Name field.
	id, name, err := util.ParseIDName(req.Name)
	if err != nil {
		log.Error(err, "failed to parse request key")
		return ctrl.Result{}, err
	}

	evaluateJob := batch.Job{}
	err = pc.client.Get(context.Background(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      name,
	}, &evaluateJob)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("try to fetch evaluatejob but it has been deleted.", "key", req.String())

			if err = pc.evaluateBackend.DeleteEvaluateJob(req.Namespace, name, id); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	//PV_OSSMap := make(map[string]map[string]string)
	PVC_OSSMap := make(map[string]string)

	//persistentVolumeList := &v1.PersistentVolumeList{}
	//if err := pc.client.List(context.TODO(), persistentVolumeList); err != nil {
	//	return ctrl.Result{}, err
	//}
	//for _, persistentVolume := range persistentVolumeList.Items {
	//	csi := persistentVolume.Spec.CSI
	//	if csi != nil {
	//		if csi.Driver == "ossplugin.csi.alibabacloud.com" {
	//			PV_OSSMap[persistentVolume.Name] = map[string]string{
	//				"SecretName": csi.NodeStageSecretRef.Name,
	//				"SecretNamespace": csi.NodeStageSecretRef.Namespace,
	//				"Bucket": csi.VolumeAttributes["bucket"],
	//				"Url": csi.VolumeAttributes["url"],
	//			}
	//		}
	//	}
	//}
	//
	//persistentVolumeClaimList := &v1.PersistentVolumeClaimList{}
	//if err := pc.client.List(context.TODO(), persistentVolumeClaimList); err != nil {
	//	return ctrl.Result{}, err
	//}
	//for _, persistentVolumeClaim := range persistentVolumeClaimList.Items {
	//	PVC_OSSMap[persistentVolumeClaim.Namespace + "," + persistentVolumeClaim.Name] = PV_OSSMap[persistentVolumeClaim.Spec.VolumeName]["Url"]
	//}

	if evaluateJob.Labels["app"] != "evaluatejob" {
		return ctrl.Result{}, nil
	}

	// Persist evaluate object into storage backend.
	if err = pc.evaluateBackend.WriteEvaluateJob(&evaluateJob, PVC_OSSMap); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

func (pc *EvaluateJobPersistController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New(controllerName, mgr, controller.Options{
		Reconciler:              pc,
		MaxConcurrentReconciles: options.CtrlConfig.MaxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return pc.IsEvaluateJob(e.Meta.GetLabels())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return pc.IsEvaluateJob(e.MetaNew.GetLabels())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return pc.IsEvaluateJob(e.Meta.GetLabels())
		},
	}

	// Watch events with event events-handler.
	if err = c.Watch(&source.Kind{Type: &batch.Job{}}, &enqueueForEvaluate{}, pred); err != nil {
		return err
	}
	return nil
}

func (pc *EvaluateJobPersistController) IsEvaluateJob(labels map[string]string) bool {
	if val, ok := labels["app"]; ok {
		if val == "evaluatejob" {
			return true
		}
	}
	return false
}
