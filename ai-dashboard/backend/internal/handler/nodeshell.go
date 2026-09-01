package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/response"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	nodeShellImage     = "registry-cn-beijing-vpc.ack.aliyuncs.com/acs/busybox:stable"
	nodeShellNamespace = "kube-ai"
	nodeShellPrefix    = "node-shell-"
	nodeShellTimeout   = 30 * time.Second // wait for the ephemeral pod to become Running
	nodeShellExecLimit = 60 * time.Second // overall per-node exec deadline (includes pod wait)
)

type NodeShellHandler struct {
	kubeClient *k8s.Client
}

func newNodeShellHandler(kubeClient *k8s.Client) *NodeShellHandler {
	return &NodeShellHandler{kubeClient: kubeClient}
}

func (h *NodeShellHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/ops/node-shell/exec", adminOnly, h.ExecOnNode)
	rg.POST("/ops/node-shell/batch-exec", adminOnly, h.BatchExecOnNodes)
}

// adminOnly is a middleware that restricts access to admin-role users only.
func adminOnly(c *gin.Context) {
	// Dev mode: skip auth check
	if os.Getenv("DISABLE_AUTH") == "true" {
		c.Next()
		return
	}
	session := sessions.Default(c)
	role, _ := session.Get(auth.SessionKeyRole).(string)
	if role != auth.RoleAdmin {
		response.Failed(c, 40300, "admin access required")
		c.Abort()
		return
	}
	c.Next()
}

type NodeExecRequest struct {
	NodeName string `json:"nodeName" binding:"required"`
	Command  string `json:"command" binding:"required"`
}

type NodeExecResult struct {
	NodeName string `json:"nodeName"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Error    string `json:"error,omitempty"`
}

type BatchExecRequest struct {
	NodeNames []string `json:"nodeNames" binding:"required"`
	Command   string   `json:"command" binding:"required"`
}

// ExecOnNode executes a command on a specific node via an ephemeral privileged pod.
func (h *NodeShellHandler) ExecOnNode(c *gin.Context) {
	var req NodeExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, response.CodeK8sError, "invalid request: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), nodeShellExecLimit)
	defer cancel()

	result := h.execOnNode(ctx, req.NodeName, req.Command)
	if result.Error != "" {
		response.OK(c, result)
		return
	}
	response.OK(c, result)
}

// BatchExecOnNodes executes the same command on multiple nodes concurrently.
func (h *NodeShellHandler) BatchExecOnNodes(c *gin.Context) {
	var req BatchExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, response.CodeK8sError, "invalid request: "+err.Error())
		return
	}

	if len(req.NodeNames) > 20 {
		response.Failed(c, response.CodeK8sError, "max 20 nodes per batch")
		return
	}

	// Deduplicate node names (preserve order) to avoid executing twice on the
	// same node and racing on the same ephemeral pod name.
	nodeNames := make([]string, 0, len(req.NodeNames))
	seen := make(map[string]bool, len(req.NodeNames))
	for _, n := range req.NodeNames {
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		nodeNames = append(nodeNames, n)
	}
	if len(nodeNames) == 0 {
		response.Failed(c, response.CodeK8sError, "no valid node names")
		return
	}

	// Execute concurrently, bounded by the request context deadline.
	ctx, cancel := context.WithTimeout(c.Request.Context(), nodeShellExecLimit)
	defer cancel()

	results := make([]NodeExecResult, len(nodeNames))
	done := make(chan int, len(nodeNames))

	for i, node := range nodeNames {
		go func(idx int, nodeName string) {
			results[idx] = h.execOnNode(ctx, nodeName, req.Command)
			done <- idx
		}(i, node)
	}

	// Wait for all
	for range nodeNames {
		<-done
	}

	response.OK(c, results)
}

func (h *NodeShellHandler) execOnNode(ctx context.Context, nodeName, command string) NodeExecResult {
	return h.execOnNodeCmd(ctx, nodeName, []string{"sh", "-c", command})
}

// execOnNodeCmd runs a command given as argv (no shell interpretation) on the
// target node through an ephemeral privileged pod + nsenter.
func (h *NodeShellHandler) execOnNodeCmd(ctx context.Context, nodeName string, cmdArgs []string) NodeExecResult {
	result := NodeExecResult{NodeName: nodeName}

	sanitized := strings.Replace(nodeName, ".", "-", -1)
	if len(sanitized) > 40 {
		sanitized = sanitized[:40]
	}
	// Random suffix avoids pod name collisions when multiple requests land in
	// the same second (e.g. batch exec on nodes with shared name prefixes).
	suffix, err := randomSuffix(3)
	if err != nil {
		result.Error = "generate pod name suffix: " + err.Error()
		return result
	}
	podName := nodeShellPrefix + sanitized + "-" + suffix

	// Create ephemeral privileged pod on the target node
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: nodeShellNamespace,
			Labels:    map[string]string{"app": "node-shell", "target-node": nodeName},
		},
		Spec: corev1.PodSpec{
			NodeName:      nodeName,
			HostPID:       true,
			HostNetwork:   true,
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "shell",
					Image:   nodeShellImage,
					Command: []string{"sleep", "60"}, // Keep alive for exec
					SecurityContext: &corev1.SecurityContext{
						Privileged: boolPtr(true),
					},
				},
			},
			// Tolerate all taints so we can land on any node
			Tolerations: []corev1.Toleration{
				{Operator: corev1.TolerationOpExists},
			},
		},
	}

	// Create pod
	_, err = h.kubeClient.Typed().CoreV1().Pods(nodeShellNamespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		result.Error = "create pod: " + err.Error()
		return result
	}

	// Always clean up
	defer func() {
		_ = h.kubeClient.Typed().CoreV1().Pods(nodeShellNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
		logrus.Debugf("node-shell pod %s deleted", podName)
	}()

	// Wait for pod to be running (bounded by both the wait timeout and ctx)
	err = wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, nodeShellTimeout, true,
		func(ctx context.Context) (bool, error) {
			p, err := h.kubeClient.Typed().CoreV1().Pods(nodeShellNamespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			return p.Status.Phase == corev1.PodRunning, nil
		})
	if err != nil {
		result.Error = "wait pod running: " + err.Error()
		return result
	}

	// Exec command via nsenter into host namespace
	nsenterCmd := append([]string{"nsenter", "--target", "1", "--mount", "--uts", "--ipc", "--net", "--pid", "--"}, cmdArgs...)

	execReq := h.kubeClient.Typed().CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(nodeShellNamespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "shell",
			Command:   nsenterCmd,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(h.kubeClient.Config(), "POST", execReq.URL())
	if err != nil {
		result.Error = "create executor: " + err.Error()
		return result
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		result.Error = err.Error()
	}

	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	return result
}

func boolPtr(b bool) *bool {
	return &b
}

// randomSuffix returns a hex string of n random bytes (2n characters).
func randomSuffix(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Ensure errors package is used
var _ = errors.IsNotFound
