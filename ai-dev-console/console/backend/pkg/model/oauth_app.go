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
    
package model

import (
	"context"
	"fmt"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/constants"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	v1 "k8s.io/api/core/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// ClusterID is ACK cluster id
	ClusterID = "kubernetes.cluster.id"

	// RegionID is aliyun region id, Ex. cn-hangzhou
	RegionID = "region.id"

	// AppNamespace is namespace of dlc on ACK
	AppNamespace = "app.namespace"

	// AppName is name of develop console
	AppName = "app.name"

	DevConsoleHost = "kube-ai.dev-console.host"

	// DefaultDomain is domain of oauth app
	DefaultDomain = "default.domain"

	EnvIngressEnable = "KUBE_DL_INGRESS_ENABLE"

	EnvKubedlHOST = "KUBE_DL_HOST"

	DevConsoleDefaultNamespace = "kube-ai"

	DevConsoleDefaultName = "ack-ai-dev-console"
)

// OAuthApp is the OAuth information struct
type OAuthApp struct {
	sync.RWMutex
	Name        string
	DisplayName string
	Context     map[string]string
}

// NewOAuthApp create new OAuthApp Object
func NewOAuthApp(name string, context map[string]string) (oAuthApp *OAuthApp) {
	oAuthApp = &OAuthApp{}
	oAuthApp.Name = name
	oAuthApp.Context = context
	return oAuthApp
}

// GetDisplayName return the display name
func (oAuthApp *OAuthApp) GetDisplayName() string {
	return oAuthApp.DisplayName
}

// GetName return the alias name of OAuthApp
func (oAuthApp *OAuthApp) GetName() string {
	return oAuthApp.Name
}

// GetAppName return the OAuthApp unique app name
func (oAuthApp *OAuthApp) GetAppName() string {
	clusterId, ok := oAuthApp.Context[ClusterID]
	if !ok {
		fmt.Printf("Domain not found in context")
	}

	return fmt.Sprintf("%s-%s", clusterId, oAuthApp.Name)
}

// GetWebURI return the dlc dashboard webui domain
func (oAuthApp *OAuthApp) GetWebURI() string {
	devConsoleHost, _ := oAuthApp.GetDevConsoleHostWithRetry(3, 3)
	//klog.Infof("got dev console host:%s err:%v", devConsoleHost, err)
	if "" != devConsoleHost {
		if !strings.HasPrefix(devConsoleHost, "http") {
			devConsoleHost = "http://" + devConsoleHost
		}
		return devConsoleHost
	}
	clusterID, ok := oAuthApp.Context[ClusterID]
	if !ok {
		fmt.Printf("Cluster id not found in context")
	}

	regionID, ok := oAuthApp.Context[RegionID]
	if !ok {
		fmt.Printf("Region id not found in context")
	}

	appName, ok := oAuthApp.Context[AppName]
	if !ok {
		fmt.Printf("App name not found in context")
	}

	return fmt.Sprintf("http://%s.%s.%s.alicontainer.com", appName, clusterID, regionID)
}

func (oAuthApp *OAuthApp) GetDevConsoleHostWithRetry(tryTimes int, tryIntervalSec int) (host string, err error) {
	oAuthApp.RLock()
	host, ok := oAuthApp.Context[DevConsoleHost]
	if ok {
		oAuthApp.RUnlock()
		return host, nil
	} else {
		oAuthApp.RUnlock()
	}

	oAuthApp.Lock()
	defer oAuthApp.Unlock()
	for i := 0; i < tryTimes; i++ {
		host, err = oAuthApp.GetDevConsoleHost()
		if err == nil && host != "" {
			break
		}
		time.Sleep(time.Duration(tryIntervalSec) * time.Second)
	}
	if "" != host {
		oAuthApp.Context[DevConsoleHost] = host
	}
	return host, nil
}

func (oAuthApp *OAuthApp) GetDevConsoleHost() (host string, err error) {
	domainFromEnv := os.Getenv(EnvIngressEnable)
	if domainFromEnv == "" {
		return os.Getenv(EnvKubedlHOST), nil //backward compatible
	}
	isEnableIngress, _ := strconv.ParseBool(domainFromEnv)
	if isEnableIngress {
		ingress := networkingv1beta1.Ingress{}
		err = clientmgr.GetCtrlClient().Get(context.TODO(),
			apitypes.NamespacedName{
				Namespace: DevConsoleDefaultNamespace,
				Name:      DevConsoleDefaultName,
			}, &ingress)
		if err != nil {
			return "", err
		}
		if len(ingress.Spec.Rules) < 1 {
			return "", nil
		}
		return ingress.Spec.Rules[0].Host, nil
	}
	service := v1.Service{}
	err = clientmgr.GetCtrlClient().Get(context.TODO(),
		apitypes.NamespacedName{
			Namespace: DevConsoleDefaultNamespace,
			Name:      DevConsoleDefaultName,
		}, &service)
	if err != nil {
		return "", err
	}
	return service.Spec.ClusterIP, nil
}

// return oauth redirect uri of
func (oAuthApp *OAuthApp) GetRedirectURI() string {
	return fmt.Sprintf("%s%s%s", oAuthApp.GetWebURI(), constants.ApiV1Routes, constants.AlicloudOauth)
}
