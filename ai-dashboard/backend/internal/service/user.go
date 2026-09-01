package service

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/model"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

const (
	UserNamespace          = "kube-ai"
	AdminClusterRole       = "kubeai-admin-clusterrole"
	ResearcherClusterRole  = "kubeai-researcher-clusterrole"
	ResearcherRole         = "kubeai-researcher-role"
	RoleAdmin              = "admin"
	KubernetesEndpointName = "kubernetes"
	DefaultClusterName     = "kubernetes"
)

var userGVR = schema.GroupVersionResource{
	Group: "data.kubeai.alibabacloud.com", Version: "v1", Resource: "users",
}

// longTokenExpirationSeconds is the requested lifetime for tokens issued via
// the TokenRequest API when no SA token secret exists (K8s >= 1.24). The API
// server may cap it to --service-account-max-token-expiration.
const longTokenExpirationSeconds int64 = 365 * 24 * 3600 // 1 year

type UserService struct {
	kubeClient *k8s.Client
	crdClient *k8s.CRDClient
}

func NewUserService(kubeClient *k8s.Client) *UserService {
	return &UserService{
		kubeClient: kubeClient,
		crdClient:  k8s.NewCRDClient(kubeClient),
	}
}

func (s *UserService) ListUsers(userName string) ([]model.User, error) {
	var list model.UserList
	if err := s.crdClient.List(userGVR, UserNamespace, &list); err != nil {
		return nil, err
	}
	if userName == "" {
		return list.Items, nil
	}
	var filtered []model.User
	for _, u := range list.Items {
		if strings.Contains(u.Spec.UserName, userName) {
			filtered = append(filtered, u)
		}
	}
	return filtered, nil
}

func (s *UserService) GetUser(userID string) (*model.User, error) {
	var user model.User
	if err := s.crdClient.Get(userGVR, UserNamespace, userID, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserService) FindUserByAliuid(aliuid string) (*model.User, error) {
	users, err := s.ListUsers("")
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if contains(u.Spec.ApiRoles, RoleAdmin) && u.Spec.Aliuid == aliuid {
			return &u, nil
		}
	}
	return nil, nil
}

func (s *UserService) CreateUser(user *model.User) error {
	spec := &user.Spec
	userID := spec.UserId
	if userID == "" {
		userID = normalizeUserID(spec.UserName)
		spec.UserId = userID
	}
	user.Name = userID
	user.Namespace = UserNamespace

	saName := genSAName(spec.UserName, userID)
	saConfig := spec.K8sServiceAccount
	if saConfig == nil {
		saConfig = &model.K8sServiceAccount{
			Namespace: UserNamespace,
		}
		spec.K8sServiceAccount = saConfig
	}

	sa, err := s.kubeClient.CreateServiceAccount(UserNamespace, saName)
	if err != nil {
		// If SA already exists, get it instead of failing
		existingSA, getErr := s.kubeClient.GetServiceAccount(saName, UserNamespace)
		if getErr != nil {
			return fmt.Errorf("create service account: %w", err)
		}
		sa = existingSA
	}
	// Ensure token secret exists (ignore "already exists" error)
	_ = s.kubeClient.CreateSecretForServiceAccount(UserNamespace, saName)
	saConfig.Name = sa.Name
	saConfig.Namespace = sa.Namespace

	applyApiRoles(spec)
	user.Spec = *spec

	if err := s.updateRoleBindings(user, nil); err != nil {
		return fmt.Errorf("update role bindings: %w", err)
	}

	return s.crdClient.CreateOrReplace(userGVR, UserNamespace, user)
}

func (s *UserService) UpdateUser(user *model.User) error {
	userID := user.Name
	found, err := s.GetUser(userID)
	if err != nil {
		return err
	}
	if found == nil {
		return s.crdClient.CreateOrReplace(userGVR, UserNamespace, user)
	}

	if found.Spec.Deletable != nil && !*found.Spec.Deletable {
		user.Spec.Deletable = found.Spec.Deletable
		if !contains(user.Spec.ApiRoles, RoleAdmin) {
			user.Spec.ApiRoles = append(user.Spec.ApiRoles, RoleAdmin)
		}
	}
	user.Spec.UserId = userID

	applyApiRoles(&user.Spec)
	if err := s.updateRoleBindings(user, found); err != nil {
		return fmt.Errorf("update role bindings: %w", err)
	}

	return s.crdClient.CreateOrReplace(userGVR, UserNamespace, user)
}

func (s *UserService) DeleteUser(userID string) error {
	if userID == "" || s == nil || s.crdClient == nil {
		return fmt.Errorf("invalid user delete request")
	}
	user, err := s.GetUser(userID)
	if err != nil {
		// Treat "not found" as already deleted instead of failing.
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if user == nil {
		return nil
	}
	if user.Spec.Deletable != nil && !*user.Spec.Deletable {
		return fmt.Errorf("user %s is not deletable", userID)
	}

	// Clean up the associated service account and its admin cluster role
	// binding. Both are optional: users without a service account must not
	// trigger a nil dereference.
	saConfig := user.Spec.K8sServiceAccount
	if saConfig != nil && saConfig.Name != "" {
		_ = s.kubeClient.Typed().CoreV1().ServiceAccounts(UserNamespace).Delete(
			context.TODO(), saConfig.Name, metav1.DeleteOptions{})
		_ = s.kubeClient.Typed().RbacV1().ClusterRoleBindings().Delete(
			context.TODO(), saConfig.Name+"-"+AdminClusterRole, metav1.DeleteOptions{})
	}

	return s.crdClient.Delete(userGVR, UserNamespace, userID)
}

func (s *UserService) GetBearerToken(userID string) (string, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return "", err
	}
	if user == nil || user.Spec.K8sServiceAccount == nil || user.Spec.K8sServiceAccount.Name == "" {
		return "", fmt.Errorf("user or service account not found")
	}
	saName := user.Spec.K8sServiceAccount.Name
	saNs := user.Spec.K8sServiceAccount.Namespace
	if saNs == "" {
		saNs = UserNamespace
	}

	token, _, _, err := s.resolveSAToken(saName, saNs)
	if err != nil {
		return "", err
	}
	return token, nil
}

// resolveSAToken returns a bearer token for the given service account, plus
// the CA bundle / default namespace when they are stored alongside the token.
// It prefers the classic SA token secret paths (naming convention first, then
// the SA's legacy .secrets field) and falls back to the TokenRequest API,
// which is required on K8s >= 1.24 clusters where no token secret exists.
func (s *UserService) resolveSAToken(saName, saNs string) (token string, ca []byte, ns string, err error) {
	if s == nil || s.kubeClient == nil {
		return "", nil, "", fmt.Errorf("user service not initialized")
	}

	// Preferred: token secret named <saName>-token (created by this platform).
	secretName := saName + "-token"
	if secret, serr := s.kubeClient.GetSecret(secretName, saNs); serr == nil && secret != nil {
		if t, ok := secret.Data["token"]; ok && len(t) > 0 {
			return string(t), secret.Data["ca.crt"], string(secret.Data["namespace"]), nil
		}
	}

	// Legacy (K8s < 1.24): token secret referenced by the SA's .secrets field.
	sa, saErr := s.kubeClient.GetServiceAccount(saName, saNs)
	if saErr != nil {
		return "", nil, "", fmt.Errorf("service account %s not found: %w", saName, saErr)
	}
	if len(sa.Secrets) > 0 {
		secret, gerr := s.kubeClient.GetSecret(sa.Secrets[0].Name, saNs)
		if gerr == nil && secret != nil {
			if t, ok := secret.Data["token"]; ok && len(t) > 0 {
				return string(t), secret.Data["ca.crt"], string(secret.Data["namespace"]), nil
			}
		}
	}

	// Fallback: TokenRequest API with a long expiration (K8s >= 1.24).
	exp := longTokenExpirationSeconds
	tr, terr := s.kubeClient.Typed().CoreV1().ServiceAccounts(saNs).CreateToken(
		context.TODO(), saName,
		&authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{ExpirationSeconds: &exp},
		},
		metav1.CreateOptions{})
	if terr != nil {
		return "", nil, "", fmt.Errorf(
			"no token secret found for service account %s (tried %s and SA secrets) and TokenRequest failed: %w",
			saName, secretName, terr)
	}
	return tr.Status.Token, nil, "", nil
}

// clusterCA returns the cluster CA bundle from the current rest config,
// used when no SA token secret carries ca.crt (TokenRequest path).
func (s *UserService) clusterCA() []byte {
	cfg := s.kubeClient.Config()
	if cfg == nil {
		return nil
	}
	if len(cfg.TLSClientConfig.CAData) > 0 {
		return cfg.TLSClientConfig.CAData
	}
	if cfg.TLSClientConfig.CAFile != "" {
		if b, err := os.ReadFile(cfg.TLSClientConfig.CAFile); err == nil {
			return b
		}
	}
	return nil
}

func (s *UserService) GenKubeConfig(userID, namespace string) (string, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return "", err
	}
	if user == nil || user.Spec.K8sServiceAccount == nil {
		return "", fmt.Errorf("user or service account not found")
	}
	saName := user.Spec.K8sServiceAccount.Name
	saNs := user.Spec.K8sServiceAccount.Namespace
	if saNs == "" {
		saNs = UserNamespace
	}

	token, caCrt, tokenNs, err := s.resolveSAToken(saName, saNs)
	if err != nil {
		return "", err
	}
	// The TokenRequest path carries no ca.crt/namespace; take the CA from the
	// current cluster config and default to the SA namespace.
	if len(caCrt) == 0 {
		caCrt = s.clusterCA()
	}
	if namespace == "" {
		namespace = tokenNs
	}
	if namespace == "" {
		namespace = saNs
	}

	host, port, err := s.kubeClient.GetEndpointAddress(KubernetesEndpointName, "default")
	if err != nil {
		return "", fmt.Errorf("get kubernetes endpoint: %w", err)
	}
	serverAddr := fmt.Sprintf("https://%s:%d", host, port)

	config := api.NewConfig()
	config.Clusters[DefaultClusterName] = &api.Cluster{
		Server:                   serverAddr,
		CertificateAuthorityData: caCrt,
	}
	config.Contexts[saName] = &api.Context{
		Cluster:   DefaultClusterName,
		Namespace: namespace,
		AuthInfo:  saName,
	}
	config.AuthInfos[saName] = &api.AuthInfo{
		Token: token,
	}
	config.CurrentContext = saName

	yaml, err := clientcmd.Write(*config)
	if err != nil {
		return "", err
	}
	return string(yaml), nil
}

func (s *UserService) updateRoleBindings(user, oldUser *model.User) error {
	saConfig := user.Spec.K8sServiceAccount
	if saConfig == nil {
		return nil
	}
	saName := saConfig.Name
	saNs := saConfig.Namespace
	if saNs == "" {
		saNs = UserNamespace
	}

	newRBs := toRoleBindings(saConfig.RoleBindings)
	oldRBs := []k8s.RoleBinding{}
	if oldUser != nil && oldUser.Spec.K8sServiceAccount != nil {
		oldRBs = toRoleBindings(oldUser.Spec.K8sServiceAccount.RoleBindings)
	}
	if err := s.kubeClient.UpdateRoleBindings(saName, saNs, newRBs, oldRBs, false); err != nil {
		return err
	}

	newCRBs := toRoleBindings(saConfig.ClusterRoleBindings)
	oldCRBs := []k8s.RoleBinding{}
	if oldUser != nil && oldUser.Spec.K8sServiceAccount != nil {
		oldCRBs = toRoleBindings(oldUser.Spec.K8sServiceAccount.ClusterRoleBindings)
	}
	return s.kubeClient.UpdateRoleBindings(saName, saNs, newCRBs, oldCRBs, true)
}

func toRoleBindings(rbs []model.RoleBinding) []k8s.RoleBinding {
	result := make([]k8s.RoleBinding, len(rbs))
	for i, rb := range rbs {
		result[i] = k8s.RoleBinding{RoleName: rb.RoleName, Namespace: rb.Namespace}
	}
	return result
}

func applyApiRoles(spec *model.UserSpec) {
	sa := spec.K8sServiceAccount
	if sa == nil {
		return
	}
	if contains(spec.ApiRoles, RoleAdmin) {
		hasAdmin := false
		for _, rb := range sa.ClusterRoleBindings {
			if rb.RoleName == AdminClusterRole {
				hasAdmin = true
				break
			}
		}
		if !hasAdmin {
			sa.ClusterRoleBindings = append(sa.ClusterRoleBindings, model.RoleBinding{RoleName: AdminClusterRole})
		}
	} else {
		filtered := sa.ClusterRoleBindings[:0]
		for _, rb := range sa.ClusterRoleBindings {
			if rb.RoleName != AdminClusterRole {
				filtered = append(filtered, rb)
			}
		}
		sa.ClusterRoleBindings = filtered
		spec.Aliuid = ""
	}
}

func genSAName(userName, userID string) string {
	return userID
}

func normalizeUserID(upn string) string {
	s := strings.ToLower(upn)
	s = strings.ReplaceAll(s, "@", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
