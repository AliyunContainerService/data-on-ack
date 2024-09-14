/*
Copyright 2019 The Alibaba Authors.

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

package apis

import (
	mpiv1alpha1 "github.com/kubeflow/arena/pkg/operators/mpi-operator/apis/kubeflow/v1alpha1"
	pytorchv1 "github.com/kubeflow/arena/pkg/operators/pytorch-operator/apis/pytorch/v1"
	tfv1 "github.com/kubeflow/arena/pkg/operators/tf-operator/apis/tensorflow/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var AddToSchemes runtime.SchemeBuilder

func init() {
	AddToSchemes = append(AddToSchemes, tfv1.AddToScheme)
	AddToSchemes = append(AddToSchemes, pytorchv1.AddToScheme)
	AddToSchemes = append(AddToSchemes, mpiv1alpha1.AddToScheme)
}

func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
