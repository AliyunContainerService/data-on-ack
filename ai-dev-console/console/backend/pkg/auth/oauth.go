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
    
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	datav1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/data/v1"
	cli "github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/client"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	authenticationapi "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/dynamic"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/constants"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"

	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	appLock        sync.RWMutex
	oauthInfo      *model.OAuthInfo
	appConfigLock  sync.RWMutex
	oauthAppConfig *model.OAuthApp
	gvr            = schema.GroupVersionResource{
		Group:    "data.kubeai.alibabacloud.com",
		Version:  "v1",
		Resource: "users",
	}
)

const (
	EnvIsIntlAccount  = "INTL_ACCOUNT"
	i18nAccessCodeUrl = "https://signin.alibabacloud.com/oauth2/v1/auth"
	accessCodeUrl     = "https://signin.aliyun.com/oauth2/v1/auth"
	oauthPath         = constants.ApiV1Routes + constants.AlicloudOauth

	SessionKeyAccountID = "accountId" // 阿里云主账号ID
	SessionKeyLoginID   = "loginId"   // RAM子账号ID
	SessionKeyName      = "name"
	SessionKeyLoginName = "loginName" // RAM子账号名字
	SessionKeyUserNS    = "namespaces"
	SessionKeyToken     = "token"
	SessionKeyRole      = "role"

	SessionValueRoleAdmin      = "admin"
	SessionValueRoleResearcher = "researcher"

	EnvAdminAidKey  = "KUBE_DL_ADMIN_AID"
	kubeAINamespace = "kube-ai"
)

type Auth interface {
	Login(c *gin.Context) error
	LoginByToken(c *gin.Context) error
	Logout(c *gin.Context) error
	GetLoginUrl(c *gin.Context) string
	GetRamRedirectUrl(c *gin.Context) (loginUrl string, err error)
	GetUserNamespace(loginName string) ([]string, error)
}

// AliCloud login
type AliCloudAuth struct {
	client        *cli.AliyunRamClient
	dynamicClient dynamic.Interface
}

var LOGININVALID = errors.New("login id is inconsistent")

func NewAliCloudAuth() (*AliCloudAuth, error) {
	restConfig := ctrl.GetConfigOrDie()
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &AliCloudAuth{
		client:        cli.GetAliyunRamClient(),
		dynamicClient: dynamicClient,
	}, nil
}

func (auth *AliCloudAuth) getRamCallbackUri(c *gin.Context, oauthInfo *model.OAuthInfo) string {
	redirectUrl := "http://" + c.Request.Host + oauthPath
	//if oauthInfo.WebAppRedirectDomain != "" {
	//	redirectUrl = oauthInfo.WebAppRedirectDomain
	//}
	return redirectUrl
}

func (auth *AliCloudAuth) LoginByToken(c *gin.Context) error {
	session := sessions.Default(c)
	token, ok := c.GetQuery("token")
	// Security fix: never log tokens.
	if !ok || token == "" {
		return fmt.Errorf("token empty")
	}
	userInfo, err := getUserInfoByToken(token)
	if nil != err {
		klog.Errorf("get user info by token err:%s", err)
		return err
	}
	b, _ := json.Marshal(userInfo)
	klog.Infof("logging in user info %s", string(b))
	session.Set(SessionKeyAccountID, userInfo.Aid)
	session.Set(SessionKeyLoginID, userInfo.Uid)
	session.Set(SessionKeyName, userInfo.Name)
	session.Set(SessionKeyLoginName, userInfo.LoginName)
	session.Set(SessionKeyToken, token)
	if len(userInfo.Namespaces) > 0 {
		session.Set(SessionKeyUserNS, userInfo.Namespaces)
	}
	session.Save()
	klog.Infof("saved user aid%p.%p:%s saved:%s", c, session, userInfo.Aid, session.Get(SessionKeyAccountID))
	return nil
}

func (auth *AliCloudAuth) Login(c *gin.Context) error {
	// oauth login
	accessCode := c.Query("code")
	if accessCode == "" {
		klog.Errorf("Login oauth get accessCode is nil, url: %s", c.FullPath())
		return errors.New("invalid parameter accessCode")
	}

	oauthInfo, err := GetOauthInfo()
	if err != nil {
		klog.Errorf("Login oauth getOauthInfo err, url: %s, err: %v", c.FullPath(), err)
		return err
	}
	redirectUrl := auth.getRamCallbackUri(c, &oauthInfo)
	token, err := getToken(oauthInfo, redirectUrl, accessCode)
	if err != nil {
		klog.Errorf("Login oauth getToken err, url: %s, err: %v", c.FullPath(), err)
		return err
	}
	ramUserInfo, err := getUserInfo(token)
	if err != nil {
		klog.Errorf("Login oauth getUserInfo err, url: %s, err: %v", c.FullPath(), err)
		return err
	}
	data, _ := json.Marshal(ramUserInfo)
	klog.Infof("ram user info %s", string(data))

	userInfo := model.UserInfo{}
	var loginId, loginName string
	var isAdminRamUser bool

	if ramUserInfo.Upn != "" {
		isAdminRamUser = false
	} else {
		isAdminRamUser = true
	}

	if isAdminRamUser {
		loginId = ramUserInfo.Aid
		loginName = ramUserInfo.LoginName
	} else {
		loginId = ramUserInfo.Upn
		loginName = ramUserInfo.Upn
	}
	loginId = strings.ToLower(
		strings.Replace(
			strings.Replace(loginId, "@", "-", -1),
			"_", "-", -1))

	userInfo = model.UserInfo{
		Aid:       ramUserInfo.Aid,
		Uid:       ramUserInfo.Uid,
		Name:      loginId,
		LoginName: loginName,
	}

	b, _ := json.Marshal(userInfo)
	klog.Infof("logging in user info %s", string(b))

	loginName = userInfo.LoginName
	if loginName == "" {
		loginName = userInfo.Upn
	}

	namespaces, err := auth.GetUserNamespace(loginName)
	if err != nil {
		klog.Warningf("User namespace not found, loginName: %s", loginName)
	} else {
		klog.Infof("User namespaces: %v", namespaces)
	}

	k8sToken, err := auth.GetUserToken(&userInfo)
	if err != nil {
		klog.Errorf("fail get to user token, loginName: %s", loginName)
		return LOGININVALID
	}

	session := sessions.Default(c)
	session.Set(SessionKeyAccountID, userInfo.Aid)  //阿里云主账号ID
	session.Set(SessionKeyLoginID, userInfo.Uid)    //RAM子账号ID
	session.Set(SessionKeyName, userInfo.Name)      //RAM账号显示名称（可选）或自定义用户名
	session.Set(SessionKeyLoginName, loginName)     //RAM账号登录名称或自定义用户名
	session.Set(SessionKeyToken, k8sToken)
	if isAdminRamUser {
		session.Set(SessionKeyRole, SessionValueRoleAdmin)
	} else {
		session.Set(SessionKeyRole, SessionValueRoleResearcher)
	}

	if len(namespaces) > 0 {
		session.Set(SessionKeyUserNS, namespaces)
	}
	klog.Infof("saved user aid:%s", userInfo.Aid)
	session.Save()
	return nil
}

func (auth *AliCloudAuth) Logout(c *gin.Context) error {
	session := sessions.Default(c)
	session.Delete(SessionKeyAccountID)
	session.Delete(SessionKeyLoginID)
	session.Delete(SessionKeyUserNS)
	session.Save()
	return nil
}

func (auth *AliCloudAuth) GetLoginUrl(c *gin.Context) (loginUrl string) {
	//return "http://localhost:8001/login"
	return "http://" + c.Request.Host + "/login"
}

func (auth *AliCloudAuth) GetRamRedirectUrl(c *gin.Context) (loginUrl string, err error) {
	if c == nil {
		klog.Errorf("GetOauthUrl invalid parameter")
		return "", errors.New("invalid parameter")
	}
	oauthInfo, err := GetOauthInfo()
	if err != nil {
		klog.Errorf("oauth failed get oauthInfo,  err:  %v ", err)
		return "", err
	}
	callbackUri := auth.getRamCallbackUri(c, &oauthInfo)

	vals := url.Values{}
	vals.Add("client_id", oauthInfo.AppId)
	vals.Add("redirect_uri", callbackUri)
	vals.Add("response_type", "code")
	rurl, _ := url.Parse(GetCodeUrl())
	rurl.RawQuery = vals.Encode()
	klog.Infof("GetLoginUrl:%s", rurl.String())
	return rurl.String(), nil
}

func isIntlAccount() bool {
	isIntlAccountEnvStr := os.Getenv(EnvIsIntlAccount)
	if "true" == strings.ToLower(isIntlAccountEnvStr) {
		return true
	}
	return false
}

func GetRamDomain(isIntl bool) string {
	oauthDomain := "oauth.vpc-proxy.aliyuncs.com"
	if isIntl {
		oauthDomain = "oauth-intl.vpc-proxy.aliyuncs.com"
	}

	if !utils.IsDomainNameAvailable(oauthDomain) {
		oauthDomain = "oauth.aliyun.com"
		if isIntl {
			oauthDomain = "oauth.alibabacloud.com"
		}
	}
	log.Infof("using ram domain:%s isIntl:%v", oauthDomain, isIntl)
	return oauthDomain
}

func GetAuthTokenUrl() string {
	return fmt.Sprintf("https://%s/v1/token", GetRamDomain(isIntlAccount()))
}

func GetCodeUrl() string {
	if isIntlAccount() {
		return i18nAccessCodeUrl
	}
	return accessCodeUrl
}

func GetUserInfoUrl() string {
	return fmt.Sprintf("https://%s/v1/userinfo", GetRamDomain(isIntlAccount()))
}

func getToken(oauthInfo model.OAuthInfo, redirectUrl string, accessCode string) (string, error) {
	params := make(map[string]string)
	params["code"] = accessCode
	params["client_id"] = oauthInfo.AppId
	params["redirect_uri"] = redirectUrl
	params["grant_type"] = "authorization_code"
	params["client_secret"] = oauthInfo.AppSecret
	status, body, err := utils.RequestWithPost(GetAuthTokenUrl(), nil, params)
	if err != nil {
		klog.Errorf("oauth failed get token, appId: %s,  err:  %v ", oauthInfo.AppId, err)
		return "", err
	}
	if status != http.StatusOK {
		klog.Errorf("oauth failed get token, appId: %s,  responseBody:  %v ", oauthInfo.AppId, body)
		return "", err
	}
	if body == "" {
		klog.Errorf("oauth response body is nil, appId: %s", oauthInfo.AppId)
		return "", errors.New(fmt.Sprintf("oauth response body is nil, appId: %s", oauthInfo.AppId))
	}
	var dat map[string]string
	err = json.Unmarshal([]byte(body), &dat)
	if err != nil {
		klog.Errorf("oauth response body json Unmarshal err, appId: %s, responseBody: %s, err: %v", oauthInfo.AppId, body, err)
		return "", err
	}

	return dat["access_token"], nil
}

func getUserNameByToken(k8sToken string) (userName string, err error) {
	// parse and verify signature
	// Security fix: never log the token.
	kubeclient := clientmgr.GetKubeClient()
	result, err := kubeclient.AuthenticationV1().TokenReviews().Create(context.TODO(), &authenticationapi.TokenReview{
		Spec: authenticationapi.TokenReviewSpec{
			Token: k8sToken,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		// Security fix: a Forbidden TokenReview must be treated as an
		// authentication failure; never derive identity from the error text.
		return "", err
	}

	if result.Status.Error != "" {
		return "", fmt.Errorf(result.Status.Error)
	}

	return result.Status.User.Username, nil

}
func GetKubeAiUserNameByK8sUserName(k8sUserName string) string {
	tmp := strings.Split(k8sUserName, ":")
	return tmp[len(tmp)-1]
}
func getUserInfoByToken(k8sToken string) (userInfo *model.UserInfo, err error) {
	// get user name by token
	userName, err := getUserNameByToken(k8sToken)
	if err != nil {
		log.Errorf("get user name by token failed:%s", err)
		return nil, err
	}
	// get saName
	userName = GetKubeAiUserNameByK8sUserName(userName)
	// get user by user name
	gvr := schema.GroupVersionResource{
		Group:    "data.kubeai.alibabacloud.com",
		Version:  "v1",
		Resource: "users",
	}

	userData, err := dynamic.NewForConfigOrDie(ctrl.GetConfigOrDie()).Resource(gvr).Namespace("kube-ai").Get(context.TODO(), userName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("get user failed err:%s", err)
		return nil, err
	}

	data, err := userData.MarshalJSON()
	if err != nil {
		log.Errorf("get user failed err:%s", err)
		return nil, err
	}

	user := datav1.User{}
	if err := json.Unmarshal(data, &user); err != nil {
		log.Errorf("get user failed err:%s", err)
		return nil, err
	}

	quotaNamespaces := make([]string, 0)
	sa := user.Spec.K8sServiceAccount
	for j := 0; j < len(sa.RoleBindings); j++ {
		quotaNamespaces = append(quotaNamespaces, sa.RoleBindings[j].Namespace)
	}
	userInfo = &model.UserInfo{
		Upn:        user.Spec.UserName,
		Uid:        user.Spec.UserId,
		Aid:        user.Spec.Aliuid,
		Name:       user.ObjectMeta.Name,
		LoginName:  user.Spec.UserName,
		Namespaces: quotaNamespaces,
	}

	//clientmgr.GetCtrlClient().Get(context.TODO(),
	//userInfo.Upn = "upn"
	//userInfo.Uid = "1983706117860305"
	//userInfo.Aid = "1983706117860305"
	//userInfo.Name = "jackwg@test.aliyunid.com"
	//userInfo.LoginName = "jackwg@test.aliyunid.com"
	//userInfo.Namespaces = append(userInfo.Namespaces, "default-group")
	return
}

func getUserInfo(accessToken string) (userInfo model.UserInfo, err error) {
	header := make(map[string]string)
	header["Authorization"] = "Bearer " + accessToken
	status, body, err := utils.RequestWithHeader(http.MethodGet, GetUserInfoUrl(), header, nil)
	if err != nil {
		klog.Errorf("oauth failed get userInfo,  err:  %v ", err)
		return userInfo, err
	}
	if status != http.StatusOK {
		klog.Errorf("oauth failed get userInfo, responseBody:  %v ", body)
		return userInfo, err
	}
	klog.Infof("user info body:%s", body)
	if body == "" {
		klog.Errorf("oauth response body is nil")
		return userInfo, errors.New(fmt.Sprintf("oauth response body is nil"))
	}
	if err = json.Unmarshal([]byte(body), &userInfo); err != nil {
		klog.Errorf("oauth failed to unmarshal userInfo, responseBody: %s, err: %v", body, err)
		return userInfo, err
	}
	return userInfo, nil
}

func GetOauthAppConfig() (*model.OAuthApp, error) {
	appConfigLock.RLock()
	if oauthAppConfig == nil {
		appConfigLock.RUnlock()
		ramClient, err := cli.GetAliyunRamClient().GetRamClient()
		if err != nil {
			return nil, fmt.Errorf("get ram client: %w", err)
		}
		oauthApp, err := GenOAuthApp(ramClient)
		if err != nil {
			return nil, fmt.Errorf("gen oauth app config: %w", err)
		}
		appConfigLock.Lock()
		defer appConfigLock.Unlock()
		oauthAppConfig = oauthApp
	} else {
		appConfigLock.RUnlock()
	}
	return oauthAppConfig, nil
}

func GetOauthInfo() (model.OAuthInfo, error) {
	//return model.OAuthInfo{
	//	AppId:                "appid",
	//	AppSecret:            "secret456",
	//	WebAppRedirectDomain: GetOauthAppConfig().GetRedirectURI(),
	//	UserInfo: model.UserInfo{
	//		Aid: "1983706117860305",
	//	},
	//}, nil
	// create web app instead of read from configMap
	if constants.IsCreateWebApp {
		appLock.RLock()
		if oauthInfo != nil {
			appLock.RUnlock()
			return *oauthInfo, nil
		} else {
			appLock.RUnlock()
		}
		ramClient, err := cli.GetAliyunRamClient().GetRamClient()
		if err != nil {
			klog.Errorf("get ram client for create web app error:%v", err)
			return model.OAuthInfo{}, err
		}
		oauthApplicationConfig, err := GetOauthAppConfig()
		if err != nil {
			return model.OAuthInfo{}, fmt.Errorf("get oauth app config: %w", err)
		}
		app, err := GetOrCreateApplication(ramClient, oauthApplicationConfig)
		if err != nil {
			klog.Errorf("get or create web app error:%v", err)
			return model.OAuthInfo{}, err
		}
		appSecret, err := GetOrCreateApplicationSecret(ramClient, app.AppId)
		if err != nil {
			klog.Errorf("get or create web app secret error:%v", err)
			return model.OAuthInfo{}, err
		}
		appLock.Lock()
		defer appLock.Unlock()
		oauthInfo = &model.OAuthInfo{
			AppId:                app.AppId,
			AppSecret:            appSecret.AppSecretValue,
			WebAppRedirectDomain: oauthApplicationConfig.GetRedirectURI(),
			UserInfo: model.UserInfo{
				Aid: os.Getenv(EnvAdminAidKey),
			},
		}
		return *oauthInfo, nil
	}

	// Get oauth app config.
	configMap := &v1.ConfigMap{}
	var err = clientmgr.GetCtrlClient().Get(context.TODO(),
		apitypes.NamespacedName{
			Namespace: constants.SystemNamespace,
			Name:      constants.SystemConfigName,
		}, configMap)
	if err != nil {
		klog.Errorf("oauth failed get oauth configMap, ns: %s, name: %s, err: %v", constants.SystemNamespace, constants.SystemConfigName, err)
		return model.OAuthInfo{}, err
	}

	oauthConfig, exists := configMap.Data["oauthConfig"]
	if !exists {
		klog.Errorf("ConfigMap key `oauthConfig` not exists")
		return model.OAuthInfo{}, fmt.Errorf("ConfigMap key `oauthConfig` not exists")
	}
	if len(oauthConfig) == 0 {
		klog.Warningf("OauthConfig is empty")
		return model.OAuthInfo{}, fmt.Errorf("OauthConfig is empty")
	}

	dat := map[string]string{}
	err = json.Unmarshal([]byte(oauthConfig), &dat)
	if err != nil {
		klog.Errorf("GetOauthInfo json Unmarshal err, oauthConfig: %s, err: %v", oauthConfig, err)
		return model.OAuthInfo{}, err
	}
	return model.GetOauthInfo(dat), nil
}

func (auth *AliCloudAuth) GetUserNamespace(loginName string) ([]string, error) {
	list, err := auth.dynamicClient.Resource(gvr).Namespace(kubeAINamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("get users crd failed, reason: %v", err)
		return nil, err
	}

	for _, item := range list.Items {
		b, err := item.MarshalJSON()
		if err != nil {
			continue
		}

		r := gjson.ParseBytes(b)
		userName := r.Get("spec").Get("userName").String()
		if loginName == userName {
			var namespaces []string
			roleBindings := r.Get("spec").Get("k8sServiceAccount").Get("roleBindings").Array()
			for _, roleBinding := range roleBindings {
				ns := roleBinding.Get("namespace").String()
				namespaces = append(namespaces, ns)
			}

			return namespaces, nil
		}
	}

	return nil, errors.New("user namespace not found")
}

func (auth *AliCloudAuth) GetUserToken(userInfo *model.UserInfo) (string, error) {
	kubeclient := clientmgr.GetKubeClient()
	loginName := userInfo.LoginName
	if loginName == "" {
		loginName = userInfo.Upn
	}
	// change login name to user name
	userName := strings.Replace(loginName, "_", "-", -1)
	userName = strings.Replace(userName, "@", "-", -1)
	userName = strings.ToLower(userName)
	sa, err := kubeclient.CoreV1().ServiceAccounts(kubeAINamespace).Get(context.TODO(), userName, metav1.GetOptions{})
	if err != nil {
		klog.Warningf("fail to get sa use login name: %v", err)
		// because, admin user use Uid as serviceaccout, check if current user is admin
		sa, err = kubeclient.CoreV1().ServiceAccounts(kubeAINamespace).Get(context.TODO(), userInfo.Uid, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("fail to get sa use uid: %v", err)
			return "", fmt.Errorf("fail to  get service account")
		}
	}
	if len(sa.Secrets) < 1 {
		// Kubernetes >= 1.24 no longer auto-creates token secrets for
		// service accounts; issue a token through the TokenRequest API.
		return requestServiceAccountToken(kubeclient, sa.Name)
	}

	secretName := sa.Secrets[0].Name

	secret, err := kubeclient.CoreV1().Secrets(kubeAINamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get secret failed, ns:%s name:%s, err:%v", kubeAINamespace, secretName, err)
		return "", err
	}
	token := string(secret.Data["token"])
	return token, nil
}

// requestServiceAccountToken issues a long-lived token for a service account
// via the TokenRequest API (needed on Kubernetes >= 1.24 where SA token
// secrets are not created automatically).
func requestServiceAccountToken(kubeclient clientset.Interface, saName string) (string, error) {
	// Request a long expiration (1 year); the apiserver may cap or reject it.
	expirationSeconds := int64(86400 * 365)
	tokenRequest := &authenticationapi.TokenRequest{
		Spec: authenticationapi.TokenRequestSpec{
			ExpirationSeconds: &expirationSeconds,
		},
	}
	result, err := kubeclient.CoreV1().ServiceAccounts(kubeAINamespace).CreateToken(context.TODO(), saName, tokenRequest, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("create token request for sa %s/%s failed: %v", kubeAINamespace, saName, err)
		return "", fmt.Errorf("create token request for service account %s/%s failed (the apiserver may reject the requested long expiration, err: %v)", kubeAINamespace, saName, err)
	}
	if result.Status.Token == "" {
		return "", fmt.Errorf("empty token returned for service account %s/%s", kubeAINamespace, saName)
	}
	return result.Status.Token, nil
}
