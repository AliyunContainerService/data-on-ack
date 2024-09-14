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
	"regexp"
	"strings"
	"sync"

	datav1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/data/v1"
	cli "github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/client"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	authenticationapi "k8s.io/api/authentication/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	log.Infof("c%p:%v", c, token)
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
	session.Set(SessionKeyAccountID, userInfo.Uid)
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
	session.Set(SessionKeyAccountID, userInfo.Name) //阿里云主账号ID
	session.Set(SessionKeyLoginID, userInfo.Name)   //RAM子账号ID
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

func getUsernameFromError(err error) string {
	re := regexp.MustCompile(`^.* User "(.*)" cannot .*$`)
	return re.ReplaceAllString(err.Error(), "$1")
}

// test token
// "eyJhbGciOiJSUzI1NiIsImtpZCI6IldmbVBub1lNNFdlWGdlQnlTcDlxX0laMGM1dTVnR3l1bkh4XzZYTE8ydWMifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJrdWJlLWFpIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImFpZGFzaGJvYXJkLTE5ODM3MDYxMTc4NjAzMDUub25hbGl5dW4uY29tLXRva2VuLW50dmRzIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQubmFtZSI6ImFpZGFzaGJvYXJkLTE5ODM3MDYxMTc4NjAzMDUub25hbGl5dW4uY29tIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQudWlkIjoiMTc5OWYxNTEtZGQ3MC00OTQzLTllODktZTY1NGJjY2M4MmIxIiwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50Omt1YmUtYWk6YWlkYXNoYm9hcmQtMTk4MzcwNjExNzg2MDMwNS5vbmFsaXl1bi5jb20ifQ.TwSUPy0cxmAx2DrugLrPx3wBBzhMZwhj7tYe5urajGZnk4nHQewIsCcT97Hh4k0P6olL7jRkIKUoC9a8SBefexdhnwaR2LROA1dfKgHvfdzhsUEKVA92wRFA7ZujMIHrn2hirz4NjsBUCOhqSAcf7rAnjJFJNQtoD6TZ5jhhPiSko_fh22FbVam1_e2G6YWFVmR88AcX8InzQA_R-64rNvrLzTG6iw6ChbQR-AMaofoqzQSNqnwyRKIPfDLRemVCWGM-6W3RzcWxSCeqVyIumjMdMH8Ym6aEXwtEQvnkVKWrJLegJTY21DC0eSTq7fUjnD26G6KhJHyIOzl8D3R5QQ",
func getUserNameByToken(k8sToken string) (userName string, err error) {
	// parse and verify signature
	log.Infof("k8stoken:[%s]", k8sToken)
	kubeclient := clientmgr.GetKubeClient()
	result, err := kubeclient.AuthenticationV1().TokenReviews().Create(context.TODO(), &authenticationapi.TokenReview{
		Spec: authenticationapi.TokenReviewSpec{
			Token: k8sToken,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsForbidden(err) {
			return getUsernameFromError(err), nil
		}
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
	json.Unmarshal([]byte(body), &userInfo)
	return userInfo, nil
}

func GetOauthAppConfig() *model.OAuthApp {
	appConfigLock.RLock()
	if oauthAppConfig == nil {
		appConfigLock.RUnlock()
		ramClient, err := cli.GetAliyunRamClient().GetRamClient()
		if err != nil {
			klog.Fatalf("get ram client error:%v", err)
			return nil
		}
		oauthApp, err := GenOAuthApp(ramClient)
		if err != nil {
			klog.Fatal("gen oauth app config failed")
			return nil
		}
		appConfigLock.Lock()
		defer appConfigLock.Unlock()
		oauthAppConfig = oauthApp
	} else {
		appConfigLock.RUnlock()
	}
	return oauthAppConfig
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
		oauthApplicationConfig := GetOauthAppConfig()
		if oauthApplicationConfig == nil {
			return model.OAuthInfo{}, errors.New("get app config failed")
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
		return "", fmt.Errorf("service account %s secret count is zero", sa)
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
