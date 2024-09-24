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

package handlers

import clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

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
