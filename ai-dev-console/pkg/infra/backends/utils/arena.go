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
	"errors"
	"fmt"
	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	"github.com/kubeflow/arena/pkg/apis/arenaclient"
	"github.com/kubeflow/arena/pkg/apis/config"
	"github.com/kubeflow/arena/pkg/apis/types"
	"github.com/tidwall/gjson"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog"
	"os"
)

const (
	kubeAINamespace = "kube-ai"
	kubeConfigPath  = "/var/kube/"
)

func GetArenaJobTypeFromKind(kind string) types.TrainingJobType {
	switch kind {
	case training.TFJobKind:
		return types.TFTrainingJob
	case training.PyTorchJobKind:
		return types.PytorchTrainingJob
	}
	return types.AllTrainingJob
}

func GetKindFromArenaJobType(typ types.TrainingJobType) string {
	switch typ {
	case types.TFTrainingJob:
		return training.TFJobKind
	case types.PytorchTrainingJob:
		return training.PyTorchJobKind
	}
	return ""
}

func GetJobStatusFromArenaStatus(status types.TrainingJobStatus) apiv1.JobConditionType {
	switch status {
	case types.TrainingJobPending:
		return apiv1.JobCreated
	case types.TrainingJobRunning:
		return apiv1.JobRunning
	case types.TrainingJobSucceeded:
		return apiv1.JobSucceeded
	case types.TrainingJobFailed:
		return apiv1.JobFailed
	}
	return ""
}

func GetJobStatusFromString(status string) apiv1.JobConditionType {
	switch status {
	case "Created":
		return apiv1.JobCreated
	case "Running":
		return apiv1.JobRunning
	case "Restarting":
		return apiv1.JobRestarting
	case "Succeeded":
		return apiv1.JobSucceeded
	case "Failed":
		return apiv1.JobFailed
	}
	return ""
}

func GenerateUserArenaClient(loginUserName string) (*arenaclient.ArenaClient, error) {
	_, filepath, err := GenerateUserKubeConfig(loginUserName, "")
	if err != nil {
		return nil, err
	}

	return clientmgr.GetArenaClientWithConfig(filepath)
}

func GenerateUserKubeConfig(loginUserName string, namespace string) ([]byte, string, error) {
	kubeConfigFile := kubeConfigPath + loginUserName

	// check if kube config exist
	_, err := os.Stat(kubeConfigFile)
	if err == nil {
		file, err := os.Open(kubeConfigFile)
		if err == nil {
			configBytes, err := ioutil.ReadAll(file)
			//klog.Infof("found local kube config file %s of user %s \n%s", kubeConfigFile, uid, string(configBytes))
			if err == nil {
				return configBytes, kubeConfigFile, nil
			}
		}
	}

	gvr := schema.GroupVersionResource{
		Group:    "data.kubeai.alibabacloud.com",
		Version:  "v1",
		Resource: "users",
	}

	restConfig := config.GetArenaConfiger().GetRestConfig()
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, "", err
	}

	users, err := dynamicClient.Resource(gvr).Namespace(kubeAINamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Infof("get users info failed, err:%v", err)
		return nil, "", err
	}

	var serviceAccountName string
	var serviceAccountNamespace string
	var userNamespace string
	found := false
	for _, item := range users.Items {
		b, _ := item.MarshalJSON()
		r := gjson.ParseBytes(b)
		userName := r.Get("spec").Get("userName").String()
		if userName == loginUserName {
			found = true
			serviceAccountName = r.Get("spec").Get("k8sServiceAccount").Get("name").String()
			serviceAccountNamespace = r.Get("spec").Get("k8sServiceAccount").Get("namespace").String()
			roleBindings := r.Get("spec").Get("k8sServiceAccount").Get("roleBindings").Array()
			if namespace != "" {
				userNamespace = namespace
			} else {
				if len(roleBindings) > 0 {
					userNamespace = roleBindings[0].Get("namespace").String()
				}
			}
		}
	}

	if !found {
		errorMsg := fmt.Sprintf("user %s has not allocated resource quota", loginUserName)
		return nil, "", errors.New(errorMsg)
	}

	sa, err := clientmgr.GetKubeClient().CoreV1().ServiceAccounts(serviceAccountNamespace).Get(context.TODO(), serviceAccountName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get serviceaccount failed, ns:%s name:%s, err:%v", serviceAccountNamespace, serviceAccountName, err)
		return nil, "", err
	}

	secretName := sa.Secrets[0].Name

	secret, err := clientmgr.GetKubeClient().CoreV1().Secrets(serviceAccountNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get secret failed, ns:%s name:%s, err:%v", serviceAccountNamespace, secretName, err)
		return nil, "", err
	}

	svc, err := clientmgr.GetKubeClient().CoreV1().Services("default").Get(context.TODO(), "kubernetes", metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get service failed, ns:default name:kubernetes, err:%v", err)
		return nil, "", err
	}

	portName := svc.Spec.Ports[0].Name
	portNum := svc.Spec.Ports[0].Port
	addrIp := svc.Spec.ClusterIP

	clusterHost := fmt.Sprintf("%s://%s:%d", portName, addrIp, portNum)

	clusters := make(map[string]*clientcmdapi.Cluster)
	clusters["default-cluster"] = &clientcmdapi.Cluster{
		Server:                   clusterHost,
		CertificateAuthorityData: secret.Data["ca.crt"],
	}

	contexts := make(map[string]*clientcmdapi.Context)
	contexts["default-context"] = &clientcmdapi.Context{
		Cluster:   "default-cluster",
		Namespace: userNamespace,
		AuthInfo:  serviceAccountName,
	}

	authInfos := make(map[string]*clientcmdapi.AuthInfo)
	authInfos[serviceAccountName] = &clientcmdapi.AuthInfo{
		Token: string(secret.Data["token"]),
	}

	clientConfig := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: "default-context",
		AuthInfos:      authInfos,
	}

	configBytes, err := clientcmd.Write(clientConfig)
	if err != nil {
		klog.Errorf("generate user %s kube config failed, err:%v", loginUserName, err)
		return nil, "", err
	}
	//klog.Infof("generate user %s kube config: \n%s", userId, string(configBytes))

	err = clientcmd.WriteToFile(clientConfig, kubeConfigFile)
	if err != nil {
		klog.Errorf("save kube config of %s to %s failed, err: %v", loginUserName, kubeConfigFile, err)
		return configBytes, "", err
	} else {
		klog.Infof("save kube config of %s to %s", loginUserName, kubeConfigFile)
	}

	return configBytes, kubeConfigFile, nil
}
