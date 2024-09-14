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

package tenant

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/utils"
	"github.com/kubeflow/arena/pkg/apis/arenaclient"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

var defaultClientRegistry = NewClientRegistry()

type ClientRegistry struct {
	lock        sync.Mutex
	arenas      map[string]*arenaclient.ArenaClient
	ctrlClients map[string]client.Client
}

func NewClientRegistry() *ClientRegistry {
	return &ClientRegistry{
		arenas:      make(map[string]*arenaclient.ArenaClient),
		ctrlClients: make(map[string]client.Client),
	}
}

func (r *ClientRegistry) AddArenaClient(userName string, arena *arenaclient.ArenaClient) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.arenas[userName] = arena
}

func (r *ClientRegistry) GetArenaClient(userName string) *arenaclient.ArenaClient {
	return r.arenas[userName]
}

func (r *ClientRegistry) AddCtrlClient(userName string, ctrlClient client.Client) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.ctrlClients[userName] = ctrlClient
}

func (r *ClientRegistry) GetCtrlClient(userName string) client.Client {
	return r.ctrlClients[userName]
}

func AddArenaClient(userName string, arena *arenaclient.ArenaClient) {
	defaultClientRegistry.AddArenaClient(userName, arena)
}

func GetArenaClient(loginUserName string) (*arenaclient.ArenaClient, error) {
	klog.Infof("get arena client of user %s", loginUserName)
	arena := defaultClientRegistry.GetArenaClient(loginUserName)
	if arena == nil {
		newArena, err := utils.GenerateUserArenaClient(loginUserName)
		if err != nil {
			klog.Errorf("failed to generate arena client of user %s, err: %v\n", loginUserName, err)
			return nil, err
		} else {
			if newArena != nil {
				arena = newArena
				AddArenaClient(loginUserName, arena)
			}
		}
	}
	return arena, nil
}

func AddCtrlClient(userName string, ctrlClient client.Client) {
	defaultClientRegistry.AddCtrlClient(userName, ctrlClient)
}

func GetCtrlClient(userName string) (client.Client, error) {
	//klog.Infof("get apiserver client of user %s", userName)
	ctrlClient := defaultClientRegistry.GetCtrlClient(userName)

	if ctrlClient == nil {
		configBytes, _, err := utils.GenerateUserKubeConfig(userName, "")
		if err != nil {
			klog.Errorf("failed to generate apiserver client of user %s, err:%v", userName, err)
			return nil, err
		}

		newCtrlClient := clientmgr.GetCtrlClientWithConfig(configBytes)
		if newCtrlClient != nil {
			ctrlClient = newCtrlClient
			AddCtrlClient(userName, ctrlClient)
		}
	}
	return ctrlClient, nil
}

func GetCtrlDynamicClient(userName string) (dynamic.Interface, error) {
	kubeConfigBytes, _, err := utils.GenerateUserKubeConfig(userName, "")
	if err != nil {
		klog.Errorf("failed to generate apiserver client of user %s, err:%v", userName, err)
		return nil, err
	}
	cfg, err := clientcmd.NewClientConfigFromBytes(kubeConfigBytes)
	if err != nil {
		klog.Errorf("failed to generate config from kube config, err:%v", err)
		return nil, err
	}
	restConfig, err := cfg.ClientConfig()
	if err != nil {
		klog.Errorf("failed to generate rest config, err:%v", err)
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("failed to init dynamic client, err:%v", err)
		return nil, err
	}
	return dynamicClient, nil
}

func GetKubernetesClient(userName string) (*kubernetes.Clientset, error) {
	kubeConfigBytes, _, err := utils.GenerateUserKubeConfig(userName, "")
	if err != nil {
		klog.Errorf("failed to generate apiserver client of user %s, err:%v", userName, err)
		return nil, err
	}
	cfg, err := clientcmd.NewClientConfigFromBytes(kubeConfigBytes)
	if err != nil {
		klog.Errorf("failed to generate config from kube config, err:%v", err)
		return nil, err
	}
	restConfig, err := cfg.ClientConfig()
	if err != nil {
		klog.Errorf("failed to generate rest config, err:%v", err)
		return nil, err
	}
	kubernetesClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("failed to init dynamic client, err:%v", err)
		return nil, err
	}
	return kubernetesClient, nil
}
