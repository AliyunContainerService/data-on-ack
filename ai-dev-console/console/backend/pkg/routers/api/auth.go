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
    
package api

import (
	"context"
	"encoding/json"
	datav1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/data/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	md "github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/middleware"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"k8s.io/klog"
)

func NewAuthAPIsController(loginAuth auth.Auth) *AuthAPIsController {
	return &AuthAPIsController{
		loginAuth: loginAuth,
	}
}

type AuthAPIsController struct {
	loginAuth auth.Auth
}

func (ac *AuthAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	routes.GET("/login/oauth2/:type", ac.login)
	routes.GET("/current-user", ac.currentUser)
	routes.GET("/logout", ac.logout)
	routes.GET("/login-by-ram", ac.loginByRam)
	routes.GET("/ingressAuth", ac.ingressAuth)
}

func (ac *AuthAPIsController) loginByRam(c *gin.Context) {
	userInfo := ac.getCurrentUser(c)
	if nil == userInfo {
		klog.Info("userInfo is nil, redirectToLogin")
		ac.redirectToLogin(c)
		return
	}
	utils.Succeed(c, userInfo)
	return
}

func (ac *AuthAPIsController) login(c *gin.Context) {
	loginType := c.Param("type")
	log.Infof("login type:%s %p", loginType, c)
	if loginType != "alicloud" && loginType != "token" {
		klog.Errorf("login not support current login type, url: %s", c.FullPath())
		utils.Redirect500(c)
		return
	}
	var err error
	if loginType == "alicloud" {
		err = ac.loginAuth.Login(c)
		if err != nil {
			klog.Errorf("login err, err: %v, url: %s", err, c.FullPath())
			utils.Redirect403(c)
			return
		}
		c.Redirect(http.StatusFound, "http://"+c.Request.Host)
		return
	}
	err = ac.loginAuth.LoginByToken(c)
	if err != nil {
		klog.Errorf("login err, err: %v, url: %s", err, c.FullPath())
		utils.Redirect403(c)
		return
	}
	session := sessions.Default(c)
	// if found logging user redirect to root /
	utils.Succeed(c, map[string]interface{}{
		"accountId":  session.Get(auth.SessionKeyAccountID),
		"loginId":    session.Get(auth.SessionKeyLoginID),
		"name":       session.Get(auth.SessionKeyName),
		"loginName":  session.Get(auth.SessionKeyLoginName),
		"namespaces": session.Get(auth.SessionKeyUserNS),
	})
	return
}

func (ac *AuthAPIsController) redirectToLogin(c *gin.Context) {
	loginUrl, err := ac.loginAuth.GetRamRedirectUrl(c)
	if err != nil {
		utils.Redirect403(c)
		return
	}
	klog.Infof("redirect to login url: %s", loginUrl)
	c.JSONP(http.StatusOK, gin.H{
		"code": http.StatusUnauthorized,
		"data": map[string]interface{}{
			"ssoRedirect": loginUrl,
		},
	})
	return
}

func (ac *AuthAPIsController) logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete(auth.SessionKeyAccountID)
	session.Delete(auth.SessionKeyLoginID)
	session.Save()
	if md.EnableAuth() {
		utils.RedirectTo(c, ac.loginAuth.GetLoginUrl(c))
	}
	return
}

func (ac *AuthAPIsController) currentUser(c *gin.Context) {
	userInfo := ac.getCurrentUser(c)
	if nil != userInfo {
		utils.Succeed(c, userInfo)
		return
	}
	if md.EnableAuth() && c.Request.URL.Path != "/login" {
		utils.RedirectTo(c, ac.loginAuth.GetLoginUrl(c))
		return
	}
	utils.Succeed(c, nil)
	return
}
func (ac *AuthAPIsController) getCurrentUser(c *gin.Context) map[string]interface{} {
	var res map[string]interface{}
	//res = map[string]interface{}{
	//	"accountId":  "1983706117860305",
	//	"loginId":    "1983706117860305",
	//	"name":       "1983706117860305",
	//	"loginName":  "jackwg@test.aliyunid.com",
	//	"namespaces": [1]string{"default-group"},
	//}
	//return res

	session := sessions.Default(c)
	accountId, ok := session.Get(auth.SessionKeyAccountID).(string)

	if !ok || "" == accountId {
		klog.Info("current user id not found")
		return nil
	}

	klog.Infof("logined account id:%s", accountId)
	var namespaces []string
	var err error
	loginName, ok := session.Get(auth.SessionKeyLoginName).(string)
	if !ok {
		klog.Info("current user login name not found")
	} else {
		namespaces, err = ac.loginAuth.GetUserNamespace(loginName)
		if err != nil {
			klog.Warningf("user %s has no namespace", loginName)
		}
		session.Set(auth.SessionKeyUserNS, namespaces)
		session.Save()
	}

	//klog.Infof("current-user %v", accountId)
	res = map[string]interface{}{
		"accountId":  session.Get(auth.SessionKeyAccountID),
		"loginId":    session.Get(auth.SessionKeyLoginID),
		"name":       session.Get(auth.SessionKeyName),
		"loginName":  loginName,
		"namespaces": namespaces,
	}
	return res
}

func (ac *AuthAPIsController) ingressAuth(c *gin.Context) {
	session := sessions.Default(c)
	v := session.Get(auth.SessionKeyAccountID)
	if v == nil {
		klog.Warningf("ingressAuth failed, SessionKeyAccountID is Nil")
		c.JSONP(http.StatusForbidden, gin.H{})
		return
	}

	oauthInfo, err := auth.GetOauthInfo()
	if err != nil {
		klog.Errorf("ingressAuth GetOauthInfo err, err: %v", err)
		c.JSONP(http.StatusInternalServerError, gin.H{})
		return
	}
	if oauthInfo.UserInfo.Aid != v.(string) {
		klog.Warningf("ingressAuth failed, aid are different SessionKeyAccountID: %v", v)
		c.JSONP(http.StatusForbidden, gin.H{})
		return
	}

	c.JSONP(http.StatusOK, gin.H{})
	return
}

func IsAdminUser(userName string) bool {
	userName = auth.GetKubeAiUserNameByK8sUserName(userName)
	// get user by userName
	gvr := schema.GroupVersionResource{
		Group:    "data.kubeai.alibabacloud.com",
		Version:  "v1",
		Resource: "users",
	}

	userData, err := dynamic.NewForConfigOrDie(ctrl.GetConfigOrDie()).Resource(gvr).Namespace("kube-ai").Get(context.TODO(), userName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("get user failed err:%s", err)
		return false
	}

	data, err := userData.MarshalJSON()
	if err != nil {
		log.Errorf("get user failed err:%s", err)
		return false
	}

	user := datav1.User{}
	if err := json.Unmarshal(data, &user); err != nil {
		log.Errorf("get user failed err:%s", err)
		return false
	}

	for _, apiRole := range user.Spec.ApiRoles {
		if apiRole == "admin" {
			return true
		}
	}
	return false
}
