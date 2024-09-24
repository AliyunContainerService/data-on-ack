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

package utils

import (
	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	mpiv1alpha1 "github.com/kubeflow/arena/pkg/operators/mpi-operator/apis/kubeflow/v1alpha1"
	pytorchv1 "github.com/kubeflow/arena/pkg/operators/pytorch-operator/apis/pytorch/v1"
	tfv1 "github.com/kubeflow/arena/pkg/operators/tf-operator/apis/tensorflow/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func InitJobRuntimeObjectByKind(kind string) client.Object {
	var (
		job client.Object
	)

	switch kind {
	case training.TFJobKind:
		job = &tfv1.TFJob{}
	case training.PyTorchJobKind:
		job = &pytorchv1.PyTorchJob{}
	case training.MPIJobKind:
		job = &mpiv1alpha1.MPIJob{}
		//case training.XDLJobKind:
		//	job = &training.XDLJob{}
		//case training.XGBoostJobKind:
		//	job = &training.XGBoostJob{}
	}

	return job
}

func InitJobMetaObjectByKind(kind string) metav1.Object {
	var (
		job metav1.Object
	)

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

	return job

}

func RuntimeObjToMetaObj(obj runtime.Object) (metaObj metav1.Object, ok bool) {
	meta, ok := obj.(metav1.Object)
	return meta, ok
}
