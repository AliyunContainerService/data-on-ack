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
    
package culler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/AliyunContainerService/data-on-ack/notebook-controller/pkg/metrics"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("culler")
var client = &http.Client{
	Timeout: time.Second * 10,
}

const DEFAULT_CULL_IDLE_TIME = "1440" // One day
const DEFAULT_IDLENESS_CHECK_PERIOD = "1"
const DEFAULT_ENABLE_CULLING = "false"
const DEFAULT_CLUSTER_DOMAIN = "cluster.local"
const DEFAULT_DEV = "false"

const STOP_ANNOTATION = "kubeflow-resource-stopped"
const LAST_ACTIVITY_ANNOTATION = "notebooks.kubeflow.org/last-activity"

const (
	KERNEL_EXECUTION_STATE_IDLE     = "idle"
	KERNEL_EXECUTION_STATE_BUSY     = "busy"
	KERNEL_EXECUTION_STATE_STARTING = "starting"
)

type KernelStatus struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	LastActivity   string `json:"last_activity"`
	ExecutionState string `json:"execution_state"`
	Connections    int    `json:"connections"`
}

func getEnvDefault(variable string, defaultVal string) string {
	envVar := os.Getenv(variable)
	if len(envVar) == 0 {
		return defaultVal
	}
	return envVar
}

func getNamespacedNameFromMeta(meta metav1.ObjectMeta) types.NamespacedName {
	return types.NamespacedName{
		Name:      meta.GetName(),
		Namespace: meta.GetNamespace(),
	}
}

func createTimestamp() string {
	now := time.Now()
	return now.Format(time.RFC3339)
}

func GetRequeueTime() time.Duration {
	cullingPeriod := getEnvDefault(
		"IDLENESS_CHECK_PERIOD", DEFAULT_IDLENESS_CHECK_PERIOD)
	realCullingPeriod, err := strconv.Atoi(cullingPeriod)
	if err != nil {
		log.Info(fmt.Sprintf(
			"Culling Period should be Int. Got '%s'. Using default value.",
			cullingPeriod))
		realCullingPeriod, _ = strconv.Atoi(DEFAULT_IDLENESS_CHECK_PERIOD)
	}

	return time.Duration(realCullingPeriod) * time.Minute
}

func getMaxIdleTime() time.Duration {
	idleTime := getEnvDefault("CULL_IDLE_TIME", DEFAULT_CULL_IDLE_TIME)
	realIdleTime, err := strconv.Atoi(idleTime)
	if err != nil {
		log.Info(fmt.Sprintf(
			"CULL_IDLE_TIME should be Int. Got %s instead. Using default value.",
			idleTime))
		realIdleTime, _ = strconv.Atoi(DEFAULT_CULL_IDLE_TIME)
	}

	return time.Minute * time.Duration(realIdleTime)
}

func SetStopAnnotation(meta *metav1.ObjectMeta, m *metrics.Metrics) {
	if meta == nil {
		log.Info("Error: Metadata is Nil. Can't set Annotations")
		return
	}
	t := time.Now()
	if meta.GetAnnotations() != nil {
		meta.Annotations[STOP_ANNOTATION] = t.Format(time.RFC3339)
	} else {
		meta.SetAnnotations(map[string]string{
			STOP_ANNOTATION: t.Format(time.RFC3339),
		})
	}
	if m != nil {
		m.NotebookCullingCount.WithLabelValues(meta.Namespace, meta.Name).Inc()
		m.NotebookCullingTimestamp.WithLabelValues(meta.Namespace, meta.Name).Set(float64(t.Unix()))
	}

	if meta.GetAnnotations() != nil {
		if _, ok := meta.GetAnnotations()["notebooks.kubeflow.org/last_activity"]; ok {
			delete(meta.GetAnnotations(), "notebooks.kubeflow.org/last_activity")
		}
	}
}

func StopAnnotationIsSet(meta metav1.ObjectMeta) bool {
	if meta.GetAnnotations() == nil {
		return false
	}

	if _, ok := meta.GetAnnotations()[STOP_ANNOTATION]; ok {
		return true
	} else {
		return false
	}
}

func getNotebookApiKernels(nm, ns string) []KernelStatus {

	domain := getEnvDefault("CLUSTER_DOMAIN", DEFAULT_CLUSTER_DOMAIN)
	url := fmt.Sprintf(
		"http://%s.%s.svc.%s/notebook/%s/%s/api/kernels",
		nm, ns, domain, ns, nm)
	if getEnvDefault("DEV", DEFAULT_DEV) != "false" {
		url = fmt.Sprintf(
			"http://localhost:8001/api/v1/namespaces/%s/services/%s:http-%s/proxy/notebook/%s/%s/api/kernels",
			ns, nm, nm, ns, nm)
	}

	resp, err := client.Get(url)
	if err != nil {
		log.Error(err, fmt.Sprintf("Error talking to %s", url))
		return nil
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Info(fmt.Sprintf(
			"Warning: GET to %s: %d", url, resp.StatusCode))
		return nil
	}

	var kernels []KernelStatus

	err = json.NewDecoder(resp.Body).Decode(&kernels)
	if err != nil {
		log.Error(err, "Error parsing JSON response for Notebook API Kernels.")
		return nil
	}

	return kernels
}

func allKernelsAreIdle(kernels []KernelStatus, log logr.Logger) bool {
	log.Info("Examining if all kernels are idle")

	if kernels == nil {
		return false
	}

	for i := 0; i < len(kernels); i++ {
		if kernels[i].ExecutionState != KERNEL_EXECUTION_STATE_IDLE {
			log.Info("Not all kernels are idle")
			return false
		}
	}
	log.Info("All kernels are idle")
	return true
}

func UpdateNotebookLastActivityAnnotation(meta *metav1.ObjectMeta) bool {
	log := log.WithValues("notebook", getNamespacedNameFromMeta(*meta))
	if meta == nil {
		log.Info("Metadata is Nil. Can't update Last Activity Annotation.")
		return false
	}

	log.Info("Updating the last-activity annotation.")
	nm, ns := meta.GetName(), meta.GetNamespace()

	if _, ok := meta.GetAnnotations()[LAST_ACTIVITY_ANNOTATION]; !ok {
		t := createTimestamp()
		log.Info(fmt.Sprintf("No last-activity found in the CR. Setting to %s", t))

		if len(meta.GetAnnotations()) == 0 {
			meta.SetAnnotations(map[string]string{})
		}
		meta.Annotations[LAST_ACTIVITY_ANNOTATION] = t
		return true
	}

	log.Info("last-activity annotation exists. Checking /api/kernels")
	kernels := getNotebookApiKernels(nm, ns)
	if kernels == nil {
		log.Info("Could not GET the kernels status. Will not update last-activity.")
		return false
	}

	return updateTimestampFromKernelsActivity(meta, kernels)
}

func updateTimestampFromKernelsActivity(meta *metav1.ObjectMeta, kernels []KernelStatus) bool {
	log := log.WithValues("notebook", getNamespacedNameFromMeta(*meta))

	if len(kernels) == 0 {
		log.Info("Notebook has no kernels. Will not update last-activity")
		return false
	}

	if !allKernelsAreIdle(kernels, log) {
		t := createTimestamp()
		log.Info(fmt.Sprintf("Found a busy kernel. Updating the last-activity to %s", t))

		meta.Annotations[LAST_ACTIVITY_ANNOTATION] = t
		return true
	}

	recentTime, err := time.Parse(time.RFC3339, kernels[0].LastActivity)
	if err != nil {
		log.Error(err, "Error parsing the last-activity from the /api/kernels")
		return false
	}

	for i := 1; i < len(kernels); i++ {
		kernelLastActivity, err := time.Parse(time.RFC3339, kernels[i].LastActivity)
		if err != nil {
			log.Error(err, "Error parsing the last-activity from the /api/kernels")
			return false
		}
		if kernelLastActivity.After(recentTime) {
			recentTime = kernelLastActivity
		}
	}
	t := recentTime.Format(time.RFC3339)

	meta.Annotations[LAST_ACTIVITY_ANNOTATION] = t
	log.Info(fmt.Sprintf("Successfully updated last-activity from latest kernel action, %s", t))
	return true
}

func notebookIsIdle(meta metav1.ObjectMeta) bool {
	log := log.WithValues("notebook", getNamespacedNameFromMeta(meta))

	if meta.GetAnnotations() != nil {
		// Read the current LAST_ACTIVITY_ANNOTATION
		tempLastActivity := meta.GetAnnotations()[LAST_ACTIVITY_ANNOTATION]
		LastActivity, err := time.Parse(time.RFC3339, tempLastActivity)
		if err != nil {
			log.Error(err, "Error parsing last-activity time")
			return false
		}

		timeCap := LastActivity.Add(getMaxIdleTime())
		if time.Now().After(timeCap) {
			return true
		}
	}
	return false
}

func NotebookNeedsCulling(meta metav1.ObjectMeta) bool {
	log := log.WithValues("notebook", getNamespacedNameFromMeta(meta))

	if getEnvDefault("ENABLE_CULLING", DEFAULT_ENABLE_CULLING) != "true" {
		log.Info("Culling of idle Pods is Disabled. To enable it set the " +
			"ENV Var 'ENABLE_CULLING=true'")
		return false
	}

	if StopAnnotationIsSet(meta) {
		log.Info("Notebook is already stopping")
		return false
	}

	return notebookIsIdle(meta)
}
