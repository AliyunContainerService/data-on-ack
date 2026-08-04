package service

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

//go:embed manifests/*.yaml
var manifestFS embed.FS

func (s *InitService) Initialize() error {
	// 1. Apply ClusterRoles
	if err := s.applyClusterRoles(); err != nil {
		return fmt.Errorf("apply cluster roles: %w", err)
	}
	// 2. Apply default ElasticQuotaTree
	if err := s.applyDefaultResource("manifests/default_eqtree.yaml", eqTreeGVR, "kube-system"); err != nil {
		return fmt.Errorf("apply default eqtree: %w", err)
	}
	// 3. Apply default UserGroup
	if err := s.applyDefaultResource("manifests/default_usergroup.yaml", userGroupGVR, "kube-ai"); err != nil {
		return fmt.Errorf("apply default user group: %w", err)
	}
	return nil
}

func (s *InitService) applyClusterRoles() error {
	roles := []string{
		"manifests/admin_clusterrole.yaml",
		"manifests/researcher_clusterrole.yaml",
	}
	for _, path := range roles {
		data, err := manifestFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		var cr rbacv1.ClusterRole
		if err := yaml.Unmarshal(data, &cr); err != nil {
			return fmt.Errorf("unmarshal %s: %w", path, err)
		}
		existing, err := s.kubeClient.Typed().RbacV1().ClusterRoles().Get(
			context.TODO(), cr.Name, metav1.GetOptions{})
		if err == nil && existing != nil {
			continue
		}
		_, err = s.kubeClient.Typed().RbacV1().ClusterRoles().Create(
			context.TODO(), &cr, metav1.CreateOptions{})
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("create cluster role %s: %w", cr.Name, err)
		}
	}

	// Apply researcher Role (namespace-scoped)
	roleData, err := manifestFS.ReadFile("manifests/researcher_role.yaml")
	if err != nil {
		return fmt.Errorf("read researcher role: %w", err)
	}
	var role rbacv1.Role
	if err := yaml.Unmarshal(roleData, &role); err != nil {
		return fmt.Errorf("unmarshal researcher role: %w", err)
	}
	for _, ns := range []string{"default", "default-group"} {
		role.Namespace = ns
		_, err = s.kubeClient.Typed().RbacV1().Roles(ns).Create(
			context.TODO(), &role, metav1.CreateOptions{})
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("create role %s in %s: %w", role.Name, ns, err)
		}
	}
	return nil
}

func (s *InitService) applyDefaultResource(path string, gvr schema.GroupVersionResource, namespace string) error {
	data, err := manifestFS.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	u, err := yamlToUnstructured(string(data))
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	name := u.GetName()
	if name == "" {
		return fmt.Errorf("resource in %s has no name", path)
	}

	_, err = s.kubeClient.Dynamic().Resource(gvr).Namespace(namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	_, err = s.kubeClient.Dynamic().Resource(gvr).Namespace(namespace).Create(
		context.TODO(), u, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("create %s: %w", name, err)
	}
	return nil
}
