package k8s

import (
	"context"
	"fmt"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	KubeAINamespace = "kube-ai"
)

// Client wraps the K8s typed and dynamic clients (admin-level).
type Client struct {
	typed   kubernetes.Interface
	dynamic dynamic.Interface
	config  *rest.Config
}

func NewClient() (*Client, error) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("get k8s rest config: %w", err)
	}

	// Ensure rate limiter is properly initialized (prevents nil pointer panics)
	cfg.QPS = 50
	cfg.Burst = 100

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

func (c *Client) Typed() kubernetes.Interface   { return c.typed }
func (c *Client) Dynamic() dynamic.Interface    { return c.dynamic }
func (c *Client) Config() *rest.Config           { return c.config }

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

// GetServiceAccountToken retrieves the token from an SA token secret.
func (c *Client) GetServiceAccountToken(saName, namespace string) (string, error) {
	secretName := saName + "-token"
	secret, err := c.typed.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get secret %s/%s: %w", namespace, secretName, err)
	}
	token, ok := secret.Data["token"]
	if !ok {
		return "", fmt.Errorf("secret %s/%s has no token data", namespace, secretName)
	}
	return string(token), nil
}

// --- RBAC helpers ---

func (c *Client) EnsureClusterRoleBinding(roleName, saName, saNamespace string) error {
	name := saName + "-" + roleName
	_, err := c.typed.RbacV1().ClusterRoleBindings().Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Subjects: []rbacv1.Subject{{
			Kind: "ServiceAccount", Name: saName, Namespace: saNamespace,
		}},
		RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: roleName},
	}
	_, err = c.typed.RbacV1().ClusterRoleBindings().Create(context.TODO(), crb, metav1.CreateOptions{})
	return err
}

func (c *Client) EnsureRoleBinding(roleName, namespace, saName, saNamespace string) error {
	name := saName + "-" + namespace + "-" + roleName
	_, err := c.typed.RbacV1().RoleBindings(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Subjects: []rbacv1.Subject{{
			Kind: "ServiceAccount", Name: saName, Namespace: saNamespace,
		}},
		RoleRef: rbacv1.RoleRef{Kind: "Role", Name: roleName},
	}
	_, err = c.typed.RbacV1().RoleBindings(namespace).Create(context.TODO(), rb, metav1.CreateOptions{})
	return err
}

// --- Cluster info ---

func (c *Client) GetClusterID() (string, error) {
	cm, err := c.typed.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "cluster-info", metav1.GetOptions{})
	if err == nil && cm != nil {
		if id, ok := cm.Data["clusterid"]; ok {
			return id, nil
		}
	}
	return "unknown", nil
}

func (c *Client) GetAPIServerHost() string {
	return c.config.Host
}

// --- CRD GVR mappings ---

func CRDGVR(name string) schema.GroupVersionResource {
	gvrMap := map[string]schema.GroupVersionResource{
		"users": {
			Group: "data.kubeai.alibabacloud.com", Version: "v1", Resource: "users",
		},
		"usergroups": {
			Group: "data.kubeai.alibabacloud.com", Version: "v1", Resource: "usergroups",
		},
		"notebooks": {
			Group: "kubeflow.org", Version: "v1", Resource: "notebooks",
		},
		"tfjobs": {
			Group: "kubeflow.org", Version: "v1", Resource: "tfjobs",
		},
		"pytorchjobs": {
			Group: "kubeflow.org", Version: "v1", Resource: "pytorchjobs",
		},
		"mpijobs": {
			Group: "kubeflow.org", Version: "v2beta1", Resource: "mpijobs",
		},
		"inferenceservices": {
			Group: "serving.kserve.io", Version: "v1beta1", Resource: "inferenceservices",
		},
		"elasticquotatrees": {
			Group: "scheduling.sigs.k8s.io", Version: "v1beta1", Resource: "elasticquotatrees",
		},
		"datasets": {
			Group: "data.fluid.io", Version: "v1alpha1", Resource: "datasets",
		},
		"configmaps": {
			Group: "", Version: "v1", Resource: "configmaps",
		},
		"persistentvolumeclaims": {
			Group: "", Version: "v1", Resource: "persistentvolumeclaims",
		},
		"rayjobs": {
			Group: "ray.io", Version: "v1", Resource: "rayjobs",
		},
		"rayclusters": {
			Group: "ray.io", Version: "v1", Resource: "rayclusters",
		},
	}
	gvr, ok := gvrMap[strings.ToLower(name)]
	if !ok {
		return schema.GroupVersionResource{}
	}
	return gvr
}

// --- Multi-tenant per-user client ---

// TenantRegistry caches per-user K8s clients based on SA tokens.
type TenantRegistry struct {
	mu      sync.RWMutex
	clients map[string]*TenantClient
	admin   *Client
}

type TenantClient struct {
	Typed   kubernetes.Interface
	Dynamic dynamic.Interface
}

func NewTenantRegistry(admin *Client) *TenantRegistry {
	return &TenantRegistry{
		clients: make(map[string]*TenantClient),
		admin:   admin,
	}
}

// GetClient returns or creates a per-user K8s client from the user's SA token.
func (r *TenantRegistry) GetClient(userName string) (*TenantClient, error) {
	r.mu.RLock()
	tc, ok := r.clients[userName]
	r.mu.RUnlock()
	if ok {
		return tc, nil
	}

	// Build user kubeconfig from SA token
	token, err := r.admin.GetServiceAccountToken(userName, KubeAINamespace)
	if err != nil {
		return nil, fmt.Errorf("get token for user %s: %w", userName, err)
	}

	restCfg, err := buildRestConfigFromToken(r.admin.GetAPIServerHost(), token)
	if err != nil {
		return nil, fmt.Errorf("build rest config for user %s: %w", userName, err)
	}

	typed, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create typed client for user %s: %w", userName, err)
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client for user %s: %w", userName, err)
	}

	tc = &TenantClient{Typed: typed, Dynamic: dyn}

	r.mu.Lock()
	r.clients[userName] = tc
	r.mu.Unlock()

	return tc, nil
}

// InvalidateClient removes a cached user client (e.g., on token refresh).
func (r *TenantRegistry) InvalidateClient(userName string) {
	r.mu.Lock()
	delete(r.clients, userName)
	r.mu.Unlock()
}

// GenerateKubeConfig generates kubeconfig YAML bytes for a user.
func (r *TenantRegistry) GenerateKubeConfig(userName string) ([]byte, error) {
	token, err := r.admin.GetServiceAccountToken(userName, KubeAINamespace)
	if err != nil {
		return nil, err
	}

	clusterID, _ := r.admin.GetClusterID()
	apiServer := r.admin.GetAPIServerHost()

	kubeConfig := clientcmdapi.NewConfig()
	kubeConfig.Clusters[clusterID] = &clientcmdapi.Cluster{
		Server:                apiServer,
		InsecureSkipTLSVerify: true,
	}
	kubeConfig.AuthInfos[userName] = &clientcmdapi.AuthInfo{
		Token: token,
	}
	kubeConfig.Contexts["default"] = &clientcmdapi.Context{
		Cluster:  clusterID,
		AuthInfo: userName,
	}
	kubeConfig.CurrentContext = "default"

	return clientcmd.Write(*kubeConfig)
}

func buildRestConfigFromToken(host, token string) (*rest.Config, error) {
	return &rest.Config{
		Host:            host,
		BearerToken:     token,
		TLSClientConfig: rest.TLSClientConfig{Insecure: true},
	}, nil
}

// NewClientForTesting builds a Client around pre-built (e.g. fake) clients.
func NewClientForTesting(typed kubernetes.Interface, dyn dynamic.Interface) *Client {
	return &Client{typed: typed, dynamic: dyn}
}
