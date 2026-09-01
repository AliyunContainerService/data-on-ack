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
    
package middleware

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/constants"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"k8s.io/klog"
	"strings"
)

func init() {
	pflag.StringVar(&enableAuth, "enable-auth", "true", "enableAuth is check user and login")
}

var enableAuth string

func EnableAuth() bool {
	return enableAuth == "true"
}

// CheckAuthMiddleware check if user login and has
func CheckAuthMiddleware(loginAuth auth.Auth) gin.HandlerFunc {
	return func(c *gin.Context) {
		var err error

		defer func() {
			if err != nil {
				klog.Errorf("[check auth] check auth failed, err: %v", err)
			}
		}()

		// Skip auth for real static resources only.
		// Security fix: appending a static-looking suffix to an API/proxy
		// path must not bypass login.
		if c.Request.URL != nil && isStaticResourcePath(c.Request.URL.Path) {
			c.Next()
			return
		}

		if c.Request.URL != nil && (strings.HasPrefix(c.Request.URL.Path, constants.ApiV1Routes+"/login") ||
			strings.HasPrefix(c.Request.URL.Path, constants.ApiV1Routes+"/ingressAuth") ||
			strings.HasPrefix(c.Request.URL.Path, constants.ApiV1Routes+"/logout") ||
			strings.HasPrefix(c.Request.URL.Path, "/login") ||
			strings.HasPrefix(c.Request.URL.Path, constants.ApiV1Routes+"/health") ||
			strings.HasPrefix(c.Request.URL.Path, constants.ApiV1Routes+"/current-user") ||
			strings.HasPrefix(c.Request.URL.Path, "/403") ||
			strings.HasPrefix(c.Request.URL.Path, "/404") ||
			strings.HasPrefix(c.Request.URL.Path, "/500") ||
			strings.HasPrefix(c.Request.URL.Path, "/1000")) {
			//klog.Infof("[check auth] request prefixed with %s, go next.", c.Request.URL.Path)
			c.Next()
			return
		}

		// index page
		loginUrl := loginAuth.GetLoginUrl(c)
		c.Set(constants.KubedlRedirectUrl, loginUrl)

		session := sessions.Default(c)
		v := session.Get(auth.SessionKeyAccountID)
		//log.Infof("session accountId %p.%p:%s", c, session, v)
		if c.Request.URL == nil || c.Request.URL.Path == "" || c.Request.URL.Path == "/" {
			if v == nil { // not login yet
				utils.RedirectTo(c, loginUrl)
				c.Abort()
				return
			}
		}

		if v != nil {
			//oauthInfo, err := auth.GetOauthInfo()
			//	if err != nil {
			//		klog.Errorf("[check auth] getOauthInfo err, url: %s, err: %v", c.FullPath(), err)
			//		utils.Redirect500(c)
			//		c.Abort()
			//		return
			//	}

			loginName, _ := session.Get(auth.SessionKeyLoginName).(string)
			namespaces, ok := session.Get(auth.SessionKeyUserNS).([]string)
			if !ok || len(namespaces) == 0 {
				namespaces, err = loginAuth.GetUserNamespace(loginName)
				if err != nil {
					log.Errorf("get user[%s] naemspace failed:%v", loginName, err)
				} else {
					if len(namespaces) > 0 {
						session.Set(auth.SessionKeyUserNS, namespaces)
					}
				}
			}

			if len(namespaces) == 0 {
				klog.Errorf("user %s have not allocated resource quota", loginName)
				utils.Redirect1000(c)
				c.Abort()
				return
			}

			//if oauthInfo.UserInfo.Aid == v.(string) {
			c.Next()
			return
			//}
			//klog.Infof("found user not match login:%s!=%s", oauthInfo.UserInfo.Aid, v)
		}

		klog.Infof("no router found redirect to %v", loginUrl)
		/*
			c.JSONP(http.StatusOK, gin.H{
				"code": http.StatusUnauthorized,
				"data": map[string]interface{}{
					"ssoRedirect": loginUrl,
				},
			})
		*/

		utils.RedirectTo(c, loginUrl)
		c.Abort()
		return
	}
}

// dynamicRoutePrefixes are path prefixes served by handlers/reverse proxies
// instead of the static file server; they must never skip authentication.
var dynamicRoutePrefixes = []string{
	constants.ApiV1Routes, // /api/v1
	"/pipeline",
	"/mlflow",
	"/ml_metadata",
	"/notebook",
	"/vscode",
	"/sd",
	"/common",
}

// staticResourceSuffixes are file suffixes of assets served from the dist dir.
var staticResourceSuffixes = []string{".js", ".css", ".png", ".ico", ".html"}

// isStaticResourcePath reports whether the path is a static resource served
// from the frontend dist directory. Only real static paths skip auth.
func isStaticResourcePath(path string) bool {
	hasStaticSuffix := false
	for _, suffix := range staticResourceSuffixes {
		if strings.HasSuffix(path, suffix) {
			hasStaticSuffix = true
			break
		}
	}
	if !hasStaticSuffix {
		return false
	}
	for _, prefix := range dynamicRoutePrefixes {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}
	return true
}
