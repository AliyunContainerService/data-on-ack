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

package utils

import (
	"context"
	"encoding/json"
	datav1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/data/v1"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
)

func GetUserByName(name string) (user datav1.User, err error) {
	gvr := schema.GroupVersionResource{
		Group:    "data.kubeai.alibabacloud.com",
		Version:  "v1",
		Resource: "users",
	}

	userData, err := dynamic.NewForConfigOrDie(ctrl.GetConfigOrDie()).Resource(gvr).Namespace("kube-ai").Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("get user failed err:%s", err)
		return datav1.User{}, err
	}

	data, err := userData.MarshalJSON()
	if err != nil {
		log.Errorf("get user failed err:%s", err)
		return datav1.User{}, err
	}

	ret := datav1.User{}
	if err := json.Unmarshal(data, &ret); err != nil {
		log.Errorf("get user failed err:%s", err)
		return datav1.User{}, err
	}
	return ret, nil
}
