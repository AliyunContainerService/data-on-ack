/*
Copyright 2021 The Alibaba Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"context"
	"errors"
	"fmt"
	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	"github.com/kubeflow/arena/pkg/apis/arenaclient"
	"github.com/kubeflow/arena/pkg/apis/config"
	"github.com/kubeflow/arena/pkg/apis/types"
	"github.com/tidwall/gjson"
	"io/ioutil"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	kubeAINamespace = "kube-ai"
	kubeConfigPath  = "/var/kube/"

	// kubeConfigFileMode is the permission enforced on generated kubeconfig
	// files: they carry tenant credentials and must stay owner-only.
	kubeConfigFileMode os.FileMode = 0600

	// tokenRequestExpiration is the lifetime requested when minting a service
	// account token via the TokenRequest API, which is the only supported way
	// to obtain SA tokens on Kubernetes >= 1.24 (token secrets are no longer
	// auto-created there).
	tokenRequestExpiration = 24 * time.Hour

	// tokenExpiryRefreshSkew refreshes TokenRequest-backed kubeconfigs a bit
	// before their real expiry to tolerate clock skew and client reuse.
	tokenExpiryRefreshSkew = 10 * time.Minute

	// tokenKubeConfigExpirySuffix names the sidecar file recording the expiry
	// of a TokenRequest-backed kubeconfig; legacy (secret-backed) kubeconfigs
	// have no sidecar and never expire.
	tokenKubeConfigExpirySuffix = ".expiry"
)

func GetArenaJobTypeFromKind(kind string) types.TrainingJobType {
	switch kind {
	case training.TFJobKind:
		return types.TFTrainingJob
	case training.PyTorchJobKind:
		return types.PytorchTrainingJob
	}
	return types.AllTrainingJob
}

func GetKindFromArenaJobType(typ types.TrainingJobType) string {
	switch typ {
	case types.TFTrainingJob:
		return training.TFJobKind
	case types.PytorchTrainingJob:
		return training.PyTorchJobKind
	}
	return ""
}

func GetJobStatusFromArenaStatus(status types.TrainingJobStatus) apiv1.JobConditionType {
	switch status {
	case types.TrainingJobPending:
		return apiv1.JobCreated
	case types.TrainingJobRunning:
		return apiv1.JobRunning
	case types.TrainingJobSucceeded:
		return apiv1.JobSucceeded
	case types.TrainingJobFailed:
		return apiv1.JobFailed
	}
	return ""
}

func GetJobStatusFromString(status string) apiv1.JobConditionType {
	switch status {
	case "Created":
		return apiv1.JobCreated
	case "Running":
		return apiv1.JobRunning
	case "Restarting":
		return apiv1.JobRestarting
	case "Succeeded":
		return apiv1.JobSucceeded
	case "Failed":
		return apiv1.JobFailed
	}
	return ""
}

// validateLoginUserName rejects user names that could escape the kubeconfig
// base directory when used as a file name component (path traversal).
func validateLoginUserName(loginUserName string) error {
	if loginUserName == "" || loginUserName == "." || loginUserName == ".." {
		return fmt.Errorf("invalid login user name %q: must not be empty or a relative path", loginUserName)
	}
	if strings.ContainsAny(loginUserName, "/\\") || strings.Contains(loginUserName, "..") {
		return fmt.Errorf("invalid login user name %q: must not contain path separators or '..'", loginUserName)
	}
	return nil
}

// userKubeConfigPath derives the on-disk kubeconfig path for a tenant user,
// guaranteeing it stays inside kubeConfigPath.
func userKubeConfigPath(loginUserName string) (string, error) {
	if err := validateLoginUserName(loginUserName); err != nil {
		return "", err
	}
	kubeConfigFile := filepath.Join(kubeConfigPath, loginUserName)
	// Defense in depth: even after Join/Clean the result must remain under
	// the base directory.
	if !strings.HasPrefix(kubeConfigFile, filepath.Clean(kubeConfigPath)+string(os.PathSeparator)) {
		return "", fmt.Errorf("kube config path of user %q escapes base dir %s", loginUserName, kubeConfigPath)
	}
	return kubeConfigFile, nil
}

// cachedKubeConfigExpired reports whether a cached kubeconfig was minted from
// a time-limited TokenRequest token that has (nearly) expired. Kubeconfigs
// backed by static SA token secrets carry no expiry sidecar and never expire.
func cachedKubeConfigExpired(kubeConfigFile string) bool {
	expiryBytes, err := ioutil.ReadFile(kubeConfigFile + tokenKubeConfigExpirySuffix)
	if err != nil {
		return false
	}
	expiry, err := time.Parse(time.RFC3339, strings.TrimSpace(string(expiryBytes)))
	if err != nil {
		return false
	}
	return time.Now().Add(tokenExpiryRefreshSkew).After(expiry)
}

// requestServiceAccountToken mints a time-limited token for the service
// account through the TokenRequest API (Kubernetes >= 1.24 compatible).
func requestServiceAccountToken(namespace, name string) (string, time.Time, error) {
	expiration := int64(tokenRequestExpiration.Seconds())
	tr, err := clientmgr.GetKubeClient().CoreV1().ServiceAccounts(namespace).CreateToken(
		context.TODO(), name,
		&authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				ExpirationSeconds: &expiration,
			},
		},
		metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("request token for serviceaccount %s/%s failed, err:%v", namespace, name, err)
		return "", time.Time{}, err
	}
	if tr.Status.Token == "" {
		return "", time.Time{}, fmt.Errorf("token request for service account %s/%s returned an empty token", namespace, name)
	}
	expiry := tr.Status.ExpirationTimestamp.Time
	if expiry.IsZero() {
		expiry = time.Now().Add(tokenRequestExpiration)
	}
	return tr.Status.Token, expiry, nil
}

// getClusterCAData fetches the cluster CA bundle for the kubeconfig. Since
// Kubernetes 1.21 every namespace carries a kube-root-ca.crt configmap, so
// the one in the service account namespace is used.
func getClusterCAData(namespace string) ([]byte, error) {
	cm, err := clientmgr.GetKubeClient().CoreV1().ConfigMaps(namespace).Get(context.TODO(), "kube-root-ca.crt", metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get kube-root-ca.crt configmap in namespace %s failed, err:%v", namespace, err)
		return nil, err
	}
	caData := []byte(cm.Data["ca.crt"])
	if len(caData) == 0 {
		return nil, fmt.Errorf("configmap %s/kube-root-ca.crt does not contain ca.crt data", namespace)
	}
	return caData, nil
}

func GenerateUserArenaClient(loginUserName string) (*arenaclient.ArenaClient, error) {
	_, filepath, err := GenerateUserKubeConfig(loginUserName, "")
	if err != nil {
		return nil, err
	}

	return clientmgr.GetArenaClientWithConfig(filepath)
}

func GenerateUserKubeConfig(loginUserName string, namespace string) ([]byte, string, error) {
	kubeConfigFile, err := userKubeConfigPath(loginUserName)
	if err != nil {
		return nil, "", err
	}

	// check if kube config exist; TokenRequest-backed kubeconfigs carry a
	// limited-lifetime token, so refresh them once (nearly) expired.
	if _, err := os.Stat(kubeConfigFile); err == nil && !cachedKubeConfigExpired(kubeConfigFile) {
		file, err := os.Open(kubeConfigFile)
		if err == nil {
			configBytes, err := ioutil.ReadAll(file)
			file.Close()
			//klog.Infof("found local kube config file %s of user %s \n%s", kubeConfigFile, uid, string(configBytes))
			if err == nil {
				return configBytes, kubeConfigFile, nil
			}
		}
	}

	gvr := schema.GroupVersionResource{
		Group:    "data.kubeai.alibabacloud.com",
		Version:  "v1",
		Resource: "users",
	}

	restConfig := config.GetArenaConfiger().GetRestConfig()
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, "", err
	}

	users, err := dynamicClient.Resource(gvr).Namespace(kubeAINamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Infof("get users info failed, err:%v", err)
		return nil, "", err
	}

	var serviceAccountName string
	var serviceAccountNamespace string
	var userNamespace string
	found := false
	for _, item := range users.Items {
		b, _ := item.MarshalJSON()
		r := gjson.ParseBytes(b)
		userName := r.Get("spec").Get("userName").String()
		if userName == loginUserName {
			found = true
			serviceAccountName = r.Get("spec").Get("k8sServiceAccount").Get("name").String()
			serviceAccountNamespace = r.Get("spec").Get("k8sServiceAccount").Get("namespace").String()
			roleBindings := r.Get("spec").Get("k8sServiceAccount").Get("roleBindings").Array()
			if namespace != "" {
				userNamespace = namespace
			} else {
				if len(roleBindings) > 0 {
					userNamespace = roleBindings[0].Get("namespace").String()
				}
			}
		}
	}

	if !found {
		errorMsg := fmt.Sprintf("user %s has not allocated resource quota", loginUserName)
		return nil, "", errors.New(errorMsg)
	}

	sa, err := clientmgr.GetKubeClient().CoreV1().ServiceAccounts(serviceAccountNamespace).Get(context.TODO(), serviceAccountName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get serviceaccount failed, ns:%s name:%s, err:%v", serviceAccountNamespace, serviceAccountName, err)
		return nil, "", err
	}

	// Resolve the token and CA data for the kubeconfig. The legacy path reads
	// the auto-created SA token secret (never expires, so cached kubeconfigs
	// stay valid); on Kubernetes >= 1.24 such secrets no longer exist, so fall
	// back to minting a bounded token through the TokenRequest API.
	var (
		token       string
		caData      []byte
		tokenExpiry time.Time
	)
	if len(sa.Secrets) > 0 {
		secretName := sa.Secrets[0].Name
		secret, err := clientmgr.GetKubeClient().CoreV1().Secrets(serviceAccountNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("get secret failed, ns:%s name:%s, err:%v", serviceAccountNamespace, secretName, err)
			return nil, "", err
		}
		if len(secret.Data["token"]) == 0 || len(secret.Data["ca.crt"]) == 0 {
			return nil, "", fmt.Errorf("secret %s/%s does not contain a valid SA token / ca.crt", serviceAccountNamespace, secretName)
		}
		token = string(secret.Data["token"])
		caData = secret.Data["ca.crt"]
	} else {
		klog.Infof("service account %s/%s has no token secret, falling back to TokenRequest API", serviceAccountNamespace, serviceAccountName)
		token, tokenExpiry, err = requestServiceAccountToken(serviceAccountNamespace, serviceAccountName)
		if err != nil {
			return nil, "", err
		}
		caData, err = getClusterCAData(serviceAccountNamespace)
		if err != nil {
			return nil, "", err
		}
	}

	svc, err := clientmgr.GetKubeClient().CoreV1().Services("default").Get(context.TODO(), "kubernetes", metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get service failed, ns:default name:kubernetes, err:%v", err)
		return nil, "", err
	}

	if len(svc.Spec.Ports) == 0 {
		return nil, "", fmt.Errorf("service default/kubernetes has no ports")
	}
	portName := svc.Spec.Ports[0].Name
	portNum := svc.Spec.Ports[0].Port
	addrIp := svc.Spec.ClusterIP

	clusterHost := fmt.Sprintf("%s://%s:%d", portName, addrIp, portNum)

	clusters := make(map[string]*clientcmdapi.Cluster)
	clusters["default-cluster"] = &clientcmdapi.Cluster{
		Server:                   clusterHost,
		CertificateAuthorityData: caData,
	}

	contexts := make(map[string]*clientcmdapi.Context)
	contexts["default-context"] = &clientcmdapi.Context{
		Cluster:   "default-cluster",
		Namespace: userNamespace,
		AuthInfo:  serviceAccountName,
	}

	authInfos := make(map[string]*clientcmdapi.AuthInfo)
	authInfos[serviceAccountName] = &clientcmdapi.AuthInfo{
		Token: token,
	}

	clientConfig := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: "default-context",
		AuthInfos:      authInfos,
	}

	configBytes, err := clientcmd.Write(clientConfig)
	if err != nil {
		klog.Errorf("generate user %s kube config failed, err:%v", loginUserName, err)
		return nil, "", err
	}
	//klog.Infof("generate user %s kube config: \n%s", userId, string(configBytes))

	err = clientcmd.WriteToFile(clientConfig, kubeConfigFile)
	if err != nil {
		klog.Errorf("save kube config of %s to %s failed, err: %v", loginUserName, kubeConfigFile, err)
		return configBytes, "", err
	}
	// The kubeconfig carries tenant credentials; enforce owner-only
	// permissions regardless of umask or clientcmd defaults.
	if err = os.Chmod(kubeConfigFile, kubeConfigFileMode); err != nil {
		klog.Warningf("chmod kube config of %s to %v failed, err: %v", loginUserName, kubeConfigFileMode, err)
	}
	// Record the expiry of TokenRequest-backed kubeconfigs so the cache check
	// above refreshes them before the bounded token lapses.
	if !tokenExpiry.IsZero() {
		expiryFile := kubeConfigFile + tokenKubeConfigExpirySuffix
		if err = ioutil.WriteFile(expiryFile, []byte(tokenExpiry.UTC().Format(time.RFC3339)), kubeConfigFileMode); err != nil {
			klog.Warningf("save kube config expiry of %s to %s failed, err: %v", loginUserName, expiryFile, err)
		}
	}
	klog.Infof("save kube config of %s to %s", loginUserName, kubeConfigFile)

	return configBytes, kubeConfigFile, nil
}
