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

package Notebook

import (
	"context"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"fmt"

	v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/notebook/v1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/cmd/options"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/registry"
	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "NotebookPersistController"
)

var log = logf.Log.WithName("notebook-persist-controller")

func NewNotebookPersistController(mgr ctrl.Manager, notebookStorage string, region string) (*NotebookPersistController, error) {
	if notebookStorage == "" {
		return nil, stderrors.New("empty cron storage backend name")
	}

	// Get pod storage backend from backends registry.
	notebookBackend := registry.GetObjectBackend(notebookStorage)
	if notebookBackend == nil {
		return nil, fmt.Errorf("cron storage backend [%s] has not registered", notebookStorage)
	}

	// Initialize cron storage backend before pod-persist-controller created.
	if err := notebookBackend.Initialize(); err != nil {
		return nil, err
	}

	return &NotebookPersistController{
		dynamicClient:   dynamic.NewForConfigOrDie(mgr.GetConfig()),
		region:          region,
		client:          mgr.GetClient(),
		notebookBackend: notebookBackend,
	}, nil
}

var _ reconcile.Reconciler = &NotebookPersistController{}

type NotebookPersistController struct {
	region          string
	client          ctrlruntime.Client
	notebookBackend backends.ObjectStorageBackend
	dynamicClient   dynamic.Interface
}

func (pc *NotebookPersistController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log.Info("starting reconciliation", "NamespacedName", req.NamespacedName)

	// Parse uid and object name from request.Name field.

	notebook := v1.Notebook{}

	err := pc.client.Get(context.Background(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}, &notebook)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("try to fetch notebook but it has been deleted.", "key", req.String())

			if err = pc.notebookBackend.DeleteNotebook(req.Namespace, req.Name); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if len(notebook.Spec.Template.Spec.InitContainers) > 0 {
		envMap := make(map[string]string)

		for _, item := range notebook.Spec.Template.Spec.InitContainers[0].Env {
			envMap[item.Name] = item.Value
		}
		base64Code := envMap["BASE64CONFIG"]
		if base64Code != "" {
			configFile, err := base64.StdEncoding.DecodeString(base64Code)
			if err != nil {
				if err := pc.notebookBackend.WriteNotebook(&notebook); err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				return ctrl.Result{}, nil
			}

			configJson, err := yaml.YAMLToJSON(configFile)
			if err != nil {
				if err := pc.notebookBackend.WriteNotebook(&notebook); err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				return ctrl.Result{}, nil
			}

			userName, err := pc.GetUserNameFromKubeConfig(configJson)
			if err != nil {
				if err := pc.notebookBackend.WriteNotebook(&notebook); err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				return ctrl.Result{}, nil
			}

			gvr := schema.GroupVersionResource{
				Group:    "data.kubeai.alibabacloud.com",
				Version:  "v1",
				Resource: "users",
			}

			userData, err := pc.dynamicClient.Resource(gvr).Namespace("kube-ai").Get(context.TODO(), userName, metav1.GetOptions{})
			if err != nil {
				fmt.Println(err.Error())
				if err := pc.notebookBackend.WriteNotebook(&notebook); err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				return ctrl.Result{}, nil
			}

			data, err := userData.MarshalJSON()
			if err != nil {
				if err := pc.notebookBackend.WriteNotebook(&notebook); err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				return ctrl.Result{}, nil
			}

			userMessage := make(map[string]interface{})

			if err := json.Unmarshal(data, &userMessage); err != nil {
				if err := pc.notebookBackend.WriteNotebook(&notebook); err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				return ctrl.Result{}, nil
			}
			if (&notebook).Labels == nil {
				(&notebook).Labels = make(map[string]string)
			}

			(&notebook).Labels["userName"] = (userMessage["spec"].(map[string]interface{}))["userName"].(string)
		}
	}

	if err := pc.notebookBackend.WriteNotebook(&notebook); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

func (pc *NotebookPersistController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New(controllerName, mgr, controller.Options{
		Reconciler:              pc,
		MaxConcurrentReconciles: options.CtrlConfig.MaxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch events with event events-handler.
	if err = c.Watch(&source.Kind{Type: &v1.Notebook{}}, &enqueueForNotebook{}); err != nil {
		return err
	}
	return nil
}

type KubeCluster struct {
	Cluster clientcmdapi.Cluster `json:"cluster"`
	Name    string               `json:"name"`
}

type KubeAuthInfo struct {
	User clientcmdapi.AuthInfo `json:"user"`
	Name string                `json:"name"`
}

type KubeContext struct {
	Context clientcmdapi.Context `json:"context"`
	Name    string               `json:"name"`
}

type KubeConfig struct {
	Kind           string                   `json:"kind,omitempty"`
	APIVersion     string                   `json:"apiVersion,omitempty"`
	Preferences    clientcmdapi.Preferences `json:"preferences"`
	Clusters       []KubeCluster            `json:"clusters"`
	AuthInfos      []KubeAuthInfo           `json:"users"`
	Contexts       []KubeContext            `json:"contexts"`
	CurrentContext string                   `json:"current-context"`
}

func (pc *NotebookPersistController) GetUserNameFromKubeConfig(data []byte) (string, error) {
	kubeConfig := KubeConfig{}
	err := json.Unmarshal(data, &kubeConfig)
	if err != nil {
		return "", err
	}
	return kubeConfig.AuthInfos[0].Name, nil
}
