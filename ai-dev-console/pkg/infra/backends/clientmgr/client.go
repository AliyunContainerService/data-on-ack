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

package clientmgr

import (
	"github.com/kubeflow/arena/pkg/apis/arenaclient"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClientManager is a set of abstract methods to get client to connect apiserver
type ClientManager interface {
	// GetKubeClient returns a client configured with the Config. The client is
	// based on the client-go library.
	GetKubeClient() clientset.Interface

	// GetCtrlClient returns a client configured with the Config. This client may
	// not be a fully "direct" client -- it may read from a cache, for
	// instance.
	GetCtrlClient() client.Client

	// GetCtrlClientWithConfig returns a client with specified kube config
	GetCtrlClientWithConfig(kubeConfig []byte) client.Client

	// GetArenaClient return arena client
	GetArenaClient() *arenaclient.ArenaClient

	// GetArenaClientWithConfig return arena client with specified kube config
	GetArenaClientWithConfig(kubeConfigPath string) (*arenaclient.ArenaClient, error)

	// GetScheme returns an initialized Scheme
	GetScheme() *runtime.Scheme

	// IndexFields adds an index with the given field name on the given object type
	// by using the given function to extract the value for that field.
	IndexField(obj runtime.Object, field string, extractValue client.IndexerFunc) error
}

var clientManager ClientManager

func InstallClientManager(mgr ClientManager) {
	clientManager = mgr
}

func GetKubeClient() clientset.Interface {
	if clientManager == nil {
		klog.Fatal("get clientMgr fail, clientMgr is nil")
	}
	return clientManager.GetKubeClient()
}

func GetCtrlClient() client.Client {
	if clientManager == nil {
		klog.Fatal("get clientMgr fail, clientMgr is nil")
	}
	return clientManager.GetCtrlClient()
}

func GetCtrlClientWithConfig(kubeConfig []byte) client.Client {
	if kubeConfig == nil {
		klog.Fatal("get clientMgr fail, kube config is empty")
	}

	if clientManager == nil {
		klog.Fatal("get clientMgr fail, clientMgr is nil")
	}

	return clientManager.GetCtrlClientWithConfig(kubeConfig)
}

func GetArenaClient() *arenaclient.ArenaClient {
	if clientManager == nil {
		klog.Fatal("get clientMgr fail, clientMgr is nil")
	}
	return clientManager.GetArenaClient()
}

func GetArenaClientWithConfig(kubeConfigFile string) (*arenaclient.ArenaClient, error) {
	if clientManager == nil {
		klog.Fatal("get clientMgr fail, clientMgr is nil")
	}

	if kubeConfigFile == "" {
		klog.Fatal("get clientMgr fail, kube config file is empty")
	}

	return clientManager.GetArenaClientWithConfig(kubeConfigFile)
}

func GetScheme() *runtime.Scheme {
	if clientManager == nil {
		klog.Fatal("get clientMgr fail, clientMgr is nil")
	}
	return clientManager.GetScheme()
}

func IndexField(obj runtime.Object, field string, extractValue client.IndexerFunc) error {
	if clientManager == nil {
		klog.Fatal("get clientMgr fail, clientMgr is nil")
	}
	return clientManager.IndexField(obj, field, extractValue)
}
