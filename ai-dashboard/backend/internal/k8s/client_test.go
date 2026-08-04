package k8s

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCRDGVR(t *testing.T) {
	tests := []struct {
		name    string
		want    schema.GroupVersionResource
	}{
		{
			"users",
			schema.GroupVersionResource{
				Group:    "data.kubeai.alibabacloud.com",
				Version:  "v1",
				Resource: "users",
			},
		},
		{
			"usergroups",
			schema.GroupVersionResource{
				Group:    "data.kubeai.alibabacloud.com",
				Version:  "v1",
				Resource: "usergroups",
			},
		},
		{
			"elasticquotatrees",
			schema.GroupVersionResource{
				Group:    "scheduling.sigs.k8s.io",
				Version:  "v1beta1",
				Resource: "elasticquotatrees",
			},
		},
		{
			"datasets",
			schema.GroupVersionResource{
				Group:    "data.fluid.io",
				Version:  "v1alpha1",
				Resource: "datasets",
			},
		},
		{
			"alluxioruntimes",
			schema.GroupVersionResource{
				Group:    "data.fluid.io",
				Version:  "v1alpha1",
				Resource: "alluxioruntimes",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CRDGVR(tt.name)
			if got != tt.want {
				t.Errorf("CRDGVR(%q) = %+v, want %+v", tt.name, got, tt.want)
			}
		})
	}
}

func TestCRDGVR_Unknown(t *testing.T) {
	got := CRDGVR("nonexistent")
	if got.Group != "" || got.Version != "" || got.Resource != "" {
		t.Errorf("CRDGVR for unknown CRD should return empty GVR, got %+v", got)
	}
}

func TestRoleBindingKey(t *testing.T) {
	tests := []struct {
		rb           RoleBinding
		isClusterRole bool
		want          string
	}{
		{RoleBinding{RoleName: "admin"}, true, "admin"},
		{RoleBinding{RoleName: "editor", Namespace: "default"}, false, "default/editor"},
		{RoleBinding{RoleName: "viewer"}, false, "/viewer"}, // empty namespace
	}
	for _, tt := range tests {
		got := roleBindingKey(tt.rb, tt.isClusterRole)
		if got != tt.want {
			t.Errorf("roleBindingKey(%+v, %v) = %q, want %q", tt.rb, tt.isClusterRole, got, tt.want)
		}
	}
}
