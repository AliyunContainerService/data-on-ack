package k8s

import (
	"context"
	"fmt"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	KubeAINamespace = "kube-ai"
)

// Client wraps the K8s typed and dynamic clients.
type Client struct {
	typed    kubernetes.Interface
	dynamic  dynamic.Interface
	config   *rest.Config
}

func NewClient() (*Client, error) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("get k8s rest config: %w", err)
	}

	typed, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create typed client: %w", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}

	return &Client{
		typed:   typed,
		dynamic: dyn,
		config:  cfg,
	}, nil
}

func (c *Client) Typed() kubernetes.Interface {
	return c.typed
}

func (c *Client) Dynamic() dynamic.Interface {
	return c.dynamic
}

func (c *Client) Config() *rest.Config {
	return c.config
}

// --- ServiceAccount / Secret helpers ---

func (c *Client) GetServiceAccount(name, namespace string) (*corev1.ServiceAccount, error) {
	return c.typed.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (c *Client) GetSecret(name, namespace string) (*corev1.Secret, error) {
	return c.typed.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (c *Client) CreateServiceAccount(namespace, name string) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return c.typed.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), sa, metav1.CreateOptions{})
}

func (c *Client) CreateSecretForServiceAccount(namespace, saName string) error {
	// K8s >= 1.24 does not auto-generate SA secrets; we create an opaque token secret
	// and link it to the SA so the token controller populates it.
	secretName := saName + "-token"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": saName,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
	_, err := c.typed.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	return err
}

// --- RBAC helpers ---

func (c *Client) EnsureClusterRoleBinding(roleName, saName, saNamespace string) error {
	name := saName + "-" + roleName
	_, err := c.typed.RbacV1().ClusterRoleBindings().Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: saNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: roleName,
		},
	}
	_, err = c.typed.RbacV1().ClusterRoleBindings().Create(context.TODO(), crb, metav1.CreateOptions{})
	return err
}

func (c *Client) UpdateRoleBindings(saName, saNamespace string, newRoleBindings, oldRoleBindings []RoleBinding, isClusterRole bool) error {
	// Create new bindings that don't exist yet
	for _, rb := range newRoleBindings {
		if err := c.createRoleBinding(saName, saNamespace, rb, isClusterRole); err != nil {
			return fmt.Errorf("create role binding %s: %w", rb.RoleName, err)
		}
	}
	// Delete old bindings that are no longer needed
	oldSet := make(map[string]bool)
	for _, rb := range oldRoleBindings {
		key := roleBindingKey(rb, isClusterRole)
		oldSet[key] = true
	}
	for _, rb := range newRoleBindings {
		oldSet[roleBindingKey(rb, isClusterRole)] = false
	}
	for key, shouldDelete := range oldSet {
		if shouldDelete {
			_ = c.deleteRoleBinding(saName, key)
		}
	}
	return nil
}

type RoleBinding struct {
	RoleName  string `json:"roleName"`
	Namespace string `json:"namespace,omitempty"`
}

func roleBindingKey(rb RoleBinding, isClusterRole bool) string {
	if isClusterRole {
		return rb.RoleName
	}
	return rb.Namespace + "/" + rb.RoleName
}

func (c *Client) createRoleBinding(saName, saNamespace string, rb RoleBinding, isClusterRole bool) error {
	name := saName + "-" + rb.RoleName
	if !isClusterRole && rb.Namespace != "" {
		name = saName + "-" + rb.Namespace + "-" + rb.RoleName
	}

	if isClusterRole {
		_, err := c.typed.RbacV1().ClusterRoleBindings().Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return nil
		}
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Subjects: []rbacv1.Subject{{
				Kind: "ServiceAccount", Name: saName, Namespace: saNamespace,
			}},
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: rb.RoleName},
		}
		_, err = c.typed.RbacV1().ClusterRoleBindings().Create(context.TODO(), crb, metav1.CreateOptions{})
		return err
	}

	ns := rb.Namespace
	if ns == "" {
		ns = saNamespace
	}
	_, err := c.typed.RbacV1().RoleBindings(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Subjects: []rbacv1.Subject{{
			Kind: "ServiceAccount", Name: saName, Namespace: saNamespace,
		}},
		RoleRef: rbacv1.RoleRef{Kind: "Role", Name: rb.RoleName},
	}
	_, err = c.typed.RbacV1().RoleBindings(ns).Create(context.TODO(), roleBinding, metav1.CreateOptions{})
	return err
}

func (c *Client) deleteRoleBinding(saName, key string) error {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 2 {
		ns := parts[0]
		name := saName + "-" + ns + "-" + parts[1]
		return c.typed.RbacV1().RoleBindings(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
	}
	name := saName + "-" + key
	return c.typed.RbacV1().ClusterRoleBindings().Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// --- CRD helpers ---

// CRDGVR returns the GroupVersionResource for a CRD by short name.
func CRDGVR(name string) schema.GroupVersionResource {
	gvrMap := map[string]schema.GroupVersionResource{
		"users": {
			Group:    "data.kubeai.alibabacloud.com",
			Version:  "v1",
			Resource: "users",
		},
		"usergroups": {
			Group:    "data.kubeai.alibabacloud.com",
			Version:  "v1",
			Resource: "usergroups",
		},
		"elasticquotatrees": {
			Group:    "scheduling.sigs.k8s.io",
			Version:  "v1beta1",
			Resource: "elasticquotatrees",
		},
		"datasets": {
			Group:    "data.fluid.io",
			Version:  "v1alpha1",
			Resource: "datasets",
		},
		"alluxioruntimes": {
			Group:    "data.fluid.io",
			Version:  "v1alpha1",
			Resource: "alluxioruntimes",
		},
		"jindoruntimes": {
			Group:    "data.fluid.io",
			Version:  "v1alpha1",
			Resource: "jindoruntimes",
		},
	}
	gvr, ok := gvrMap[strings.ToLower(name)]
	if !ok {
		return schema.GroupVersionResource{}
	}
	return gvr
}

// --- Cluster info ---

func (c *Client) GetClusterID() (string, error) {
	// Read cluster ID from the kube-system namespace's kube-root-ca ConfigMap
	// or from the API server discovery. Fall back to "unknown".
	// ACK clusters embed the cluster ID in infrastructure annotations.
	cm, err := c.typed.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "cluster-info", metav1.GetOptions{})
	if err == nil && cm != nil {
		if id, ok := cm.Data["clusterid"]; ok {
			return id, nil
		}
	}
	return "unknown", nil
}

func (c *Client) GetK8sVersion() (string, error) {
	version, err := c.typed.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return version.GitVersion, nil
}

func (c *Client) GetIngressHost(name, namespace string) (string, error) {
	ing, err := c.typed.NetworkingV1().Ingresses(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if len(ing.Status.LoadBalancer.Ingress) == 0 {
		return "", fmt.Errorf("ingress %s/%s has no loadBalancer ingress", namespace, name)
	}
	return ing.Status.LoadBalancer.Ingress[0].IP, nil
}

func (c *Client) GetServiceClusterIP(name, namespace string) (string, error) {
	svc, err := c.typed.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return svc.Spec.ClusterIP, nil
}

func (c *Client) GetEndpointAddress(name, namespace string) (string, int32, error) {
	eps, err := c.typed.CoreV1().Endpoints(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return "", 0, err
	}
	if len(eps.Subsets) == 0 || len(eps.Subsets[0].Addresses) == 0 || len(eps.Subsets[0].Ports) == 0 {
		return "", 0, fmt.Errorf("endpoints %s/%s not ready", namespace, name)
	}
	return eps.Subsets[0].Addresses[0].IP, eps.Subsets[0].Ports[0].Port, nil
}
