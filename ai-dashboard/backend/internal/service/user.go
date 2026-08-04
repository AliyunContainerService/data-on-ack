package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/model"
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
	user, err := s.GetUser(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return nil
	}
	if user.Spec.Deletable != nil && !*user.Spec.Deletable {
		return fmt.Errorf("user %s is not deletable", userID)
	}

	saConfig := user.Spec.K8sServiceAccount
	if saConfig != nil && saConfig.Name != "" {
		_ = s.kubeClient.Typed().CoreV1().ServiceAccounts(UserNamespace).Delete(
			context.TODO(), saConfig.Name, metav1.DeleteOptions{})
	}

	_ = s.kubeClient.Typed().RbacV1().ClusterRoleBindings().Delete(
		context.TODO(), saConfig.Name+"-"+AdminClusterRole, metav1.DeleteOptions{})

	return s.crdClient.Delete(userGVR, UserNamespace, userID)
}

func (s *UserService) GetBearerToken(userID string) (string, error) {
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

	// K8s 1.24+: SA no longer auto-populates .secrets field.
	// Look for the token secret by naming convention: <saName>-token
	secretName := saName + "-token"
	secret, err := s.kubeClient.GetSecret(secretName, saNs)
	if err != nil {
		// Fallback: try legacy .secrets field on the SA
		sa, saErr := s.kubeClient.GetServiceAccount(saName, saNs)
		if saErr != nil {
			return "", fmt.Errorf("service account %s not found: %w", saName, saErr)
		}
		if len(sa.Secrets) == 0 {
			return "", fmt.Errorf("no token secret found for service account %s (tried %s)", saName, secretName)
		}
		secret, err = s.kubeClient.GetSecret(sa.Secrets[0].Name, saNs)
		if err != nil {
			return "", fmt.Errorf("get secret %s: %w", sa.Secrets[0].Name, err)
		}
	}

	tokenBytes, ok := secret.Data["token"]
	if !ok {
		return "", fmt.Errorf("token data not found in secret %s", secret.Name)
	}
	return string(tokenBytes), nil
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

	sa, err := s.kubeClient.GetServiceAccount(saName, saNs)
	if err != nil {
		return "", err
	}
	if len(sa.Secrets) == 0 {
		return "", fmt.Errorf("no secrets for sa %s", saName)
	}
	secret, err := s.kubeClient.GetSecret(sa.Secrets[0].Name, saNs)
	if err != nil {
		return "", err
	}

	caCrt := secret.Data["ca.crt"] // already base64-decoded by client-go
	token := string(secret.Data["token"])
	if namespace == "" {
		namespace = string(secret.Data["namespace"])
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
