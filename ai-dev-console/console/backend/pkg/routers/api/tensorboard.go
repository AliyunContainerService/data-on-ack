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
limitations under the License.
*/

package api

import (
	"context"
	"encoding/json"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/gin-gonic/gin"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// TensorBoardAPIsController serves TensorBoard status for training jobs on the
// legacy stack. The TensorBoard lifecycle operator was removed from this repo,
// so `status` is read-only (derived from the job's tensorboard-config
// annotation) and `reapply` is answered with an explicit error instead of
// silently doing nothing.
type TensorBoardAPIsController struct{}

func NewTensorBoardAPIsController() *TensorBoardAPIsController {
	return &TensorBoardAPIsController{}
}

func (tc *TensorBoardAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	routes.GET("/tensorboard/status", tc.GetStatus)
	routes.POST("/tensorboard/reapply", tc.Reapply)
}

// trainingJobKinds are the kinds living in training.kubedl.io/v1alpha1.
var trainingJobKinds = map[string]bool{
	"TFJob":      true,
	"PyTorchJob": true,
	"XGBoostJob": true,
	"XDLJob":   true,
	"MPIJob":     true,
	"MarsJob":    true,
}

func (tc *TensorBoardAPIsController) getJob(c *gin.Context) (*unstructured.Unstructured, bool) {
	namespace := c.Query("job_namespace")
	name := c.Query("job_name")
	kind := c.Query("kind")
	if namespace == "" || name == "" {
		utils.Failed(c, "job_namespace and job_name are required")
		return nil, false
	}
	if !trainingJobKinds[kind] {
		utils.Failed(c, "unsupported job kind: "+kind)
		return nil, false
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "training.kubedl.io",
		Version: "v1alpha1",
		Kind:    kind,
	})
	err := clientmgr.GetCtrlClient().Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, u)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			utils.Failed(c, "job not found")
			return nil, false
		}
		utils.Failed(c, "get job failed: "+err.Error())
		return nil, false
	}
	return u, true
}

// GetStatus reports whether a TensorBoard configuration is attached to the job.
func (tc *TensorBoardAPIsController) GetStatus(c *gin.Context) {
	u, ok := tc.getJob(c)
	if !ok {
		return
	}

	annotations := u.GetAnnotations()
	rawConfig := annotations[v1.AnnotationTensorBoardConfig]

	status := map[string]interface{}{
		"tensorboardEnabled": rawConfig != "",
	}
	if rawConfig != "" {
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(rawConfig), &cfg); err == nil {
			status["tensorboardConfig"] = cfg
		} else {
			status["tensorboardConfig"] = rawConfig
		}
	}
	utils.Succeed(c, status)
}

// Reapply is not supported on the legacy stack: the TensorBoard controller was
// removed together with the operator. Fail loudly instead of pretending success.
func (tc *TensorBoardAPIsController) Reapply(c *gin.Context) {
	utils.Failed(c, "tensorboard lifecycle management is not available in this console build (operator removed); only read-only status is supported")
}
