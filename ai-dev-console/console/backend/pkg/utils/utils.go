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

import "fmt"

type requestTarget string
type proxyGroupType string

const (
	NotebookPod requestTarget = "pod"
	NotebookSvc requestTarget = "svc"

	JupyterProxy         proxyGroupType = "notebook"
	VSCodeProxy          proxyGroupType = "vscode"
	StableDiffusionProxy proxyGroupType = "sd"
	CommonPortProxy      proxyGroupType = "common"
)

func GetCacheKey(namespace, name string, target requestTarget) string {
	return fmt.Sprintf("%s-%s-%s", namespace, name, target)
}

func GetProxyCacheKey(namespace, name string, target proxyGroupType) string {
	return fmt.Sprintf("%s-%s-%s", namespace, name, target)
}
