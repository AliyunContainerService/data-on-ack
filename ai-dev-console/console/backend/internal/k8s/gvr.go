package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetIngressHostByRules returns the first host from the ingress spec rules.
func (c *Client) GetIngressHostByRules(name, namespace string) (string, error) {
	ing, err := c.typed.NetworkingV1().Ingresses(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get ingress %s/%s: %w", namespace, name, err)
	}
	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" {
			return rule.Host, nil
		}
	}
	return "", fmt.Errorf("ingress %s/%s has no host rules", namespace, name)
}

// DeploymentGVR returns the GVR for Deployments.
func DeploymentGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}
}

// ServiceGVR returns the GVR for Services.
func ServiceGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}
}

// PodGVR returns the GVR for Pods.
func PodGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
}
