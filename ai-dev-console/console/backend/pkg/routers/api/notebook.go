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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/proxy"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"k8s.io/klog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"reflect"
	"strings"
)

func NewNotebookAPIsController(notebookHandler *handlers.NotebookHandler) *NotebookAPIsController {
	return &NotebookAPIsController{
		notebookHandler: notebookHandler,
		proxyCache:      proxy.NewSyncMapCache(reflect.TypeOf(&httputil.ReverseProxy{})),
	}
}

type NotebookAPIsController struct {
	notebookHandler *handlers.NotebookHandler
	proxyCache      *proxy.SyncMapCache
}

// allowedCommonProxyPorts whitelists the service ports the /common proxy may
// forward to. Security fix: clients must not be able to proxy arbitrary ports.
var allowedCommonProxyPorts = map[string]bool{
	"80":   true,
	"443":  true,
	"8080": true,
	"8888": true,
	"6006": true,
}

func (nc *NotebookAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	routes.POST("/notebook/create", nc.SubmitNotebook)
	routes.GET("/notebook/list", nc.GetNotebookList)
	// Bug fix: the stop/start routes were disabled so the frontend always got 404.
	routes.GET("/notebook/stop", nc.StopNotebook)
	routes.GET("/notebook/start", nc.StartNotebook)
	routes.GET("/notebook/delete", nc.DeleteNotebookByName)
	routes.GET("/notebook/maxGpu", nc.GetAvailableGpu)
	routes.GET("/notebook/listPVC", nc.GetAvailablePVCList)
	if os.Getenv("LIST_ALL_NOTEBOOKS") == "true" {
		routes.GET("/notebook/listFromStorage", nc.GetNotebookList)
	} else {
		routes.GET("/notebook/listFromStorage", nc.GetNotebookListFromStorage)
	}

	routes.GET("/notebook/sync", nc.SyncNotebooks)

}

func (nc *NotebookAPIsController) GetAvailableGpu(c *gin.Context) {
	maxGpu, err := nc.notebookHandler.GetMaxGpuNumbers()
	if err != nil {
		log.Errorf("GetAvailableGpu err : %s", err.Error())
		utils.Failed(c, err.Error())
		return
	}
	utils.Succeed(c, maxGpu/1000)
}

func (nc *NotebookAPIsController) RegisterJupyterReverseProxy(routes *gin.RouterGroup) {
	routes.GET("/*path", nc.JupyterReverseProxy)
	routes.POST("/*path", nc.JupyterReverseProxy)
	routes.PUT("/*path", nc.JupyterReverseProxy)
	//routes.OPTION("/*path", nc.NotebookReverseProxy)
	routes.DELETE("/*path", nc.JupyterReverseProxy)
	routes.PATCH("/*path", nc.JupyterReverseProxy)
	routes.HEAD("/*path", nc.JupyterReverseProxy)
}

func (nc *NotebookAPIsController) RegisterVSCodeReverseProxy(routes *gin.RouterGroup) {
	routes.GET("/*path", nc.VSCodeReverseProxy)
	routes.POST("/*path", nc.VSCodeReverseProxy)
	routes.PUT("/*path", nc.VSCodeReverseProxy)
	//routes.OPTION("/*path", nc.NotebookReverseProxy)
	routes.DELETE("/*path", nc.VSCodeReverseProxy)
	routes.PATCH("/*path", nc.VSCodeReverseProxy)
	routes.HEAD("/*path", nc.VSCodeReverseProxy)
}

// RegisterSDReverseProxy For YunQi23 stable diffusion demo
func (nc *NotebookAPIsController) RegisterSDReverseProxy(routes *gin.RouterGroup) {
	routes.Any("/*path", nc.SdReverseProxy)
}

func (nc *NotebookAPIsController) RegisterAllReverseProxy(routes *gin.RouterGroup, reverseProxyFunc func(c *gin.Context)) {
	// proxy all request to notebook pod
	routes.Any("/*path", reverseProxyFunc)
}

// sessionNamespaces returns the namespaces allocated to the logged-in user
// (taken from the login session) and whether the user is an admin.
func sessionNamespaces(c *gin.Context) ([]string, bool) {
	session := sessions.Default(c)
	if session == nil {
		return nil, false
	}
	loginName, _ := session.Get(auth.SessionKeyLoginName).(string)
	accountId, _ := session.Get(auth.SessionKeyAccountID).(string)

	// Admin users can access all namespaces
	if IsAdminUser(loginName) || IsAdminUser(accountId) {
		return nil, true
	}

	userNamespaces, _ := session.Get(auth.SessionKeyUserNS).([]string)
	return userNamespaces, false
}

// checkNotebookNamespaceOwnership verifies that the authenticated user has access
// to the specified namespace. Admin users can access all namespaces.
func checkNotebookNamespaceOwnership(c *gin.Context, namespace string) bool {
	userNamespaces, isAdmin := sessionNamespaces(c)
	if isAdmin {
		return true
	}
	for _, ns := range userNamespaces {
		if ns == namespace {
			return true
		}
	}
	return false
}

// filterNamespacesBySession intersects the requested namespaces with the
// namespaces allocated to the logged-in user, so users can only operate on
// their own namespaces. Admin users keep all requested namespaces.
func filterNamespacesBySession(c *gin.Context, requested []string) []string {
	userNamespaces, isAdmin := sessionNamespaces(c)
	if isAdmin {
		return requested
	}
	if len(userNamespaces) == 0 {
		return nil
	}
	allowed := make(map[string]bool, len(userNamespaces))
	for _, ns := range userNamespaces {
		allowed[ns] = true
	}
	filtered := make([]string, 0, len(requested))
	for _, ns := range requested {
		if allowed[ns] {
			filtered = append(filtered, ns)
		}
	}
	return filtered
}

// generateNotebookToken returns a random access token generated server-side
// with crypto/rand. Security fix: client-supplied tokens are never trusted.
func generateNotebookToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (nc *NotebookAPIsController) GetNotebookListFromStorage(c *gin.Context) {
	namespacesStr := c.Query("namespaces")
	var namespaces []string
	if err := json.Unmarshal([]byte(namespacesStr), &namespaces); err != nil {
		klog.Errorf("unmarshal namespace error:%s", err)
		utils.Failed(c, fmt.Sprintf("Namespace format error:%s", namespacesStr))
		return
	}
	// Security fix: identity must come from the login session; the
	// userName/userId query parameters are not trusted.
	session := sessions.Default(c)
	userName, _ := session.Get(auth.SessionKeyLoginName).(string)
	uid, _ := session.Get(auth.SessionKeyLoginID).(string)
	// Security fix: only operate on namespaces allocated to the session user.
	namespaces = filterNamespacesBySession(c, namespaces)
	namespacesMap := make(map[string]bool)
	for _, namespace := range namespaces {
		namespacesMap[namespace] = true
	}
	var resNotebookList []handlers.NotebookMessage
	if IsAdminUser(userName) || IsAdminUser(uid) {
		userName, uid = "", ""
	}
	for namespace := range namespacesMap {
		notebookList, err := nc.notebookHandler.ListNotebookFromStorage(namespace, userName, uid, c)
		if err != nil {
			klog.Errorf("list notebook in namespace err:%s", err)
			utils.Failed(c, err.Error())
			return
		}
		resNotebookList = append(resNotebookList, notebookList...)
	}
	utils.Succeed(c, resNotebookList)
}

func (nc *NotebookAPIsController) SyncNotebooks(c *gin.Context) {
	namespacesStr := c.Query("namespaces")
	var namespaces []string
	if err := json.Unmarshal([]byte(namespacesStr), &namespaces); err != nil {
		klog.Errorf("unmarshal namespace error:%s", err)
		utils.Failed(c, fmt.Sprintf("Namespace format error:%s", namespacesStr))
		return
	}
	// Security fix: only sync namespaces allocated to the session user.
	namespaces = filterNamespacesBySession(c, namespaces)
	for _, namespace := range namespaces {
		err := nc.notebookHandler.CompatibleNotebook(namespace)
		if err != nil {
			klog.Errorf("list notebook in namespace err:%s", err)
			utils.Failed(c, err.Error())
			return
		}
	}
	utils.Succeed(c, nil)
}

func (nc *NotebookAPIsController) GetAvailablePVCList(c *gin.Context) {
	namespace := c.Query("namespace")
	if namespace == "" {
		log.Error("Namespace is Empty.")
		utils.Failed(c, "Namespace is Empty.")
		return
	}
	// Security fix: only list PVCs in namespaces owned by the session user.
	if !checkNotebookNamespaceOwnership(c, namespace) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "access denied: you do not have permission to access this namespace"})
		c.Abort()
		return
	}

	pvcs, err := nc.notebookHandler.ListPVC(namespace)
	if err != nil {
		log.Errorf("GetPVCList err : %s", err.Error())
		utils.Failed(c, err.Error())
		return
	}
	utils.Succeed(c, pvcs)
}

func (nc *NotebookAPIsController) DeleteNotebookByName(c *gin.Context) {
	namespace := c.Query("namespace")
	name := c.Query("name")
	if namespace == "" {
		log.Error("Namespace is Empty.")
		utils.Failed(c, "Namespace is Empty.")
		return
	}
	if name == "" {
		log.Error("Name is Empty.")
		utils.Failed(c, "Name is Empty.")
		return
	}

	// Security fix: only delete notebooks in namespaces owned by the session user.
	if !checkNotebookNamespaceOwnership(c, namespace) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "access denied: you do not have permission to delete this notebook"})
		c.Abort()
		return
	}

	nc.proxyCache.Delete(utils.GetProxyCacheKey(namespace, name, utils.JupyterProxy))
	nc.proxyCache.Delete(utils.GetProxyCacheKey(namespace, name, utils.VSCodeProxy))
	nc.proxyCache.Delete(utils.GetProxyCacheKey(namespace, name, utils.StableDiffusionProxy))
	nc.proxyCache.Delete(utils.GetProxyCacheKey(namespace, name, utils.CommonPortProxy))
	log.Infof("delete notebook name: %s namespace: %s", name, namespace)
	err := nc.notebookHandler.DeleteNotebook(name, namespace)
	if err != nil {
		log.Errorf("DeleteNotebook err : %s", err.Error())
		utils.Failed(c, err.Error())
		return
	}
	utils.Succeed(c, "Delete success!")
}

func (nc *NotebookAPIsController) StopNotebook(c *gin.Context) {
	namespace := c.Query("namespace")
	name := c.Query("name")
	if namespace == "" || name == "" {
		utils.Failed(c, "Namespace or Name is Empty.")
		return
	}
	// Security fix: only stop notebooks in namespaces owned by the session user.
	if !checkNotebookNamespaceOwnership(c, namespace) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "access denied: you do not have permission to stop this notebook"})
		c.Abort()
		return
	}
	log.Infof("stop notebook name: %s namespace: %s", name, namespace)
	if err := nc.notebookHandler.StopNotebook(name, namespace); err != nil {
		log.Errorf("StopNotebook err : %s", err.Error())
		utils.Failed(c, err.Error())
		return
	}
	utils.Succeed(c, "Stop success!")
}

func (nc *NotebookAPIsController) StartNotebook(c *gin.Context) {
	namespace := c.Query("namespace")
	name := c.Query("name")
	if namespace == "" || name == "" {
		utils.Failed(c, "Namespace or Name is Empty.")
		return
	}
	// Security fix: only start notebooks in namespaces owned by the session user.
	if !checkNotebookNamespaceOwnership(c, namespace) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "access denied: you do not have permission to start this notebook"})
		c.Abort()
		return
	}
	log.Infof("start notebook name: %s namespace: %s", name, namespace)
	if err := nc.notebookHandler.StartNotebook(name, namespace); err != nil {
		log.Errorf("StartNotebook err : %s", err.Error())
		utils.Failed(c, err.Error())
		return
	}
	utils.Succeed(c, "Start success!")
}

func (nc *NotebookAPIsController) GetNotebookList(c *gin.Context) {
	namespacesStr := c.Query("namespaces")
	if namespacesStr == "" {
		utils.Failed(c, "Namespaces is Empty.")
		return
	}
	var namespaces []string
	if err := json.Unmarshal([]byte(namespacesStr), &namespaces); err != nil {
		klog.Errorf("unmarshal namespace error:%s", err)
		utils.Failed(c, fmt.Sprintf("Namespace format error:%s", namespacesStr))
		return
	}
	// Security fix: only list namespaces allocated to the session user.
	namespaces = filterNamespacesBySession(c, namespaces)
	var resNotebookList []handlers.NotebookMessage
	for _, namespace := range namespaces {
		notebookList, err := nc.notebookHandler.ListNotebook(namespace)
		if err != nil {
			klog.Errorf("list notbook in namespace err:%s", err)
			utils.Failed(c, err.Error())
			return
		}
		resNotebookList = append(resNotebookList, notebookList...)
	}
	utils.Succeed(c, resNotebookList)
}

func (nc *NotebookAPIsController) JupyterReverseProxy(c *gin.Context) {
	path := c.Param("path")

	pathArr := strings.Split(path, "/")
	if len(pathArr) < 3 {
		utils.Failed(c, "path error.")
		return
	}
	c.Request.URL.Path = fmt.Sprintf("%s%s", "notebook", path)
	c.Request.RequestURI = fmt.Sprintf("%s%s", "notebook", path)
	c.Writer.Header().Del("Content-Type")
	if len(pathArr) > 3 && pathArr[3] == "static" {
		c.Writer.Header().Del("Cache-Control")
		c.Writer.Header().Add("Cache-Control", "max-age=315360000")
	}

	namespace, name := pathArr[1], pathArr[2]

	if !checkNotebookNamespaceOwnership(c, namespace) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "access denied: you do not have permission to access this notebook"})
		c.Abort()
		return
	}

	cacheKey := utils.GetProxyCacheKey(namespace, name, utils.JupyterProxy)
	proxy, ok := nc.proxyCache.Get(cacheKey)
	if ok {
		proxy.(*httputil.ReverseProxy).ServeHTTP(c.Writer, c.Request)
	} else {
		target, err := nc.notebookHandler.GetNotebookServiceConnection(namespace, name)
		if err != nil {
			log.Errorf("Jupyter ReverseProxy err : %s", err.Error())
			utils.Failed(c, err.Error())
			return
		}

		remote, err := url.Parse(fmt.Sprintf("http://%s:%s", target, "80"))
		if err != nil {
			log.Errorf("ReverseProxy parse url err : %s", err.Error())
			utils.Failed(c, err.Error())
			return
		}

		klog.Infof("New jupyter notebook connection key: %s to remote: %s", cacheKey, target)
		reverseProxy := httputil.NewSingleHostReverseProxy(remote)
		nc.proxyCache.Store(cacheKey, reverseProxy)
		reverseProxy.ServeHTTP(c.Writer, c.Request)
	}
}

func (nc *NotebookAPIsController) VSCodeReverseProxy(c *gin.Context) {
	path := c.Param("path")

	pathArr := strings.Split(path, "/")
	if len(pathArr) < 3 {
		utils.Failed(c, "path error.")
		return
	}
	c.Request.URL.Path = strings.Join(pathArr[3:], "/")
	c.Request.RequestURI = strings.Join(pathArr[3:], "/")
	c.Writer.Header().Del("Content-Type")
	if len(pathArr) > 3 && pathArr[3] == "static" {
		c.Writer.Header().Del("Cache-Control")
		c.Writer.Header().Add("Cache-Control", "max-age=315360000")
	}

	namespace, name := pathArr[1], pathArr[2]

	if !checkNotebookNamespaceOwnership(c, namespace) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "access denied: you do not have permission to access this notebook"})
		c.Abort()
		return
	}

	cacheKey := utils.GetProxyCacheKey(namespace, name, utils.VSCodeProxy)
	proxy, ok := nc.proxyCache.Get(cacheKey)
	if ok {
		proxy.(*httputil.ReverseProxy).ServeHTTP(c.Writer, c.Request)
	} else {
		target, err := nc.notebookHandler.GetNotebookServiceConnection(namespace, name)
		if err != nil {
			log.Errorf("VSCode ReverseProxy err : %s", err.Error())
			utils.Failed(c, err.Error())
			return
		}

		remote, err := url.Parse(fmt.Sprintf("http://%s:%s", target, "80"))
		if err != nil {
			log.Errorf("ReverseProxy parse url err : %s", err.Error())
			utils.Failed(c, err.Error())
			return
		}

		klog.Infof("New vscode notebook connection key: %s to remote: %s", cacheKey, target)
		reverseProxy := httputil.NewSingleHostReverseProxy(remote)
		nc.proxyCache.Store(cacheKey, reverseProxy)
		reverseProxy.ServeHTTP(c.Writer, c.Request)
	}
}

func (nc *NotebookAPIsController) SdReverseProxy(c *gin.Context) {
	path := c.Param("path")

	pathArr := strings.Split(path, "/")
	if len(pathArr) < 3 {
		utils.Failed(c, "path error.")
		return
	}
	c.Request.URL.Path = strings.Join(pathArr[3:], "/")
	c.Request.RequestURI = strings.Join(pathArr[3:], "/")
	c.Writer.Header().Del("Content-Type")
	if len(pathArr) > 3 && pathArr[3] == "static" {
		c.Writer.Header().Del("Cache-Control")
		c.Writer.Header().Add("Cache-Control", "max-age=315360000")
	}

	namespace, name := pathArr[1], pathArr[2]

	if !checkNotebookNamespaceOwnership(c, namespace) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "access denied: you do not have permission to access this notebook"})
		c.Abort()
		return
	}

	cacheKey := utils.GetProxyCacheKey(namespace, name, utils.StableDiffusionProxy)
	proxy, ok := nc.proxyCache.Get(cacheKey)
	if ok {
		proxy.(*httputil.ReverseProxy).ServeHTTP(c.Writer, c.Request)
	} else {
		target, err := nc.notebookHandler.GetNotebookPodConnection(namespace, name)
		if err != nil {
			log.Errorf("VSCode ReverseProxy err : %s", err.Error())
			utils.Failed(c, err.Error())
			return
		}

		remote, err := url.Parse(fmt.Sprintf("http://%s:%s", target, "7860"))
		if err != nil {
			log.Errorf("ReverseProxy parse url err : %s", err.Error())
			utils.Failed(c, err.Error())
			return
		}

		klog.Infof("New stable-diffusion notebook connection key: %s to remote: %s", cacheKey, target)
		reverseProxy := httputil.NewSingleHostReverseProxy(remote)
		nc.proxyCache.Store(cacheKey, reverseProxy)
		reverseProxy.ServeHTTP(c.Writer, c.Request)
	}
}

func (nc *NotebookAPIsController) CommonReverseProxy(c *gin.Context) {
	path := c.Param("path")

	pathArr := strings.Split(path, "/")
	if len(pathArr) < 4 {
		utils.Failed(c, "path error.")
		return
	}
	c.Request.URL.Path = strings.Join(pathArr[4:], "/")
	c.Request.RequestURI = strings.Join(pathArr[4:], "/")
	c.Writer.Header().Del("Content-Type")
	if len(pathArr) > 4 && pathArr[4] == "static" {
		c.Writer.Header().Del("Cache-Control")
		c.Writer.Header().Add("Cache-Control", "max-age=315360000")
	}

	namespace, name, port := pathArr[1], pathArr[2], pathArr[3]

	// Security fix: only whitelisted service ports may be proxied, reject
	// arbitrary ports.
	if !allowedCommonProxyPorts[port] {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "access denied: port is not allowed"})
		c.Abort()
		return
	}

	if !checkNotebookNamespaceOwnership(c, namespace) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "access denied: you do not have permission to access this notebook"})
		c.Abort()
		return
	}

	cacheKey := utils.GetProxyCacheKey(namespace, name, utils.CommonPortProxy)
	proxy, ok := nc.proxyCache.Get(cacheKey)
	if ok {
		proxy.(*httputil.ReverseProxy).ServeHTTP(c.Writer, c.Request)
	} else {
		target, err := nc.notebookHandler.GetNotebookPodConnection(namespace, name)
		if err != nil {
			log.Errorf("VSCode ReverseProxy err : %s", err.Error())
			utils.Failed(c, err.Error())
			return
		}

		remote, err := url.Parse(fmt.Sprintf("http://%s:%s", target, port))
		if err != nil {
			log.Errorf("ReverseProxy parse url err : %s", err.Error())
			utils.Failed(c, err.Error())
			return
		}

		klog.Infof("New stable-diffusion notebook connection key: %s to remote: %s", cacheKey, target)
		reverseProxy := httputil.NewSingleHostReverseProxy(remote)
		nc.proxyCache.Store(cacheKey, reverseProxy)
		reverseProxy.ServeHTTP(c.Writer, c.Request)
	}
}

func (nc *NotebookAPIsController) SubmitNotebook(c *gin.Context) {
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("SubmitNotebook err : %s", err.Error())
		utils.Failed(c, err.Error())
		return
	}
	message := &handlers.NotebookSubmitData{}
	err = json.Unmarshal(data, message)
	if err != nil {
		log.Errorf("SubmitNotebook err : %s", err.Error())
		utils.Failed(c, err.Error())
		return
	}

	// Security fix: identity must come from the login session. The
	// Namespace/UserName/UserId/Token fields in the request body are never
	// trusted.
	session := sessions.Default(c)
	loginName, _ := session.Get(auth.SessionKeyLoginName).(string)
	loginID, _ := session.Get(auth.SessionKeyLoginID).(string)
	if loginName == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "user not login"})
		c.Abort()
		return
	}
	if message.Namespace == "" {
		utils.Failed(c, "Namespace is Empty.")
		return
	}
	// The requested namespace must be allocated to the session user,
	// otherwise reject with 403 (admin users may use any namespace).
	if !checkNotebookNamespaceOwnership(c, message.Namespace) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "access denied: namespace is not allocated to current user"})
		c.Abort()
		return
	}
	message.UserName = loginName
	message.UserId = loginID
	// The notebook access token must be generated server-side; any
	// client-supplied value is ignored.
	message.Token, err = generateNotebookToken()
	if err != nil {
		log.Errorf("SubmitNotebook generate token err : %s", err.Error())
		utils.Failed(c, err.Error())
		return
	}

	if message.NodeSelectors == nil {
		message.NodeSelectors = map[string]string{}
	}
	if message.Labels == nil {
		message.Labels = map[string]string{}
	}
	if message.Annotations == nil {
		message.Annotations = map[string]string{}
	}
	if message.Tolerates == nil {
		message.Tolerates = map[string]dmo.TolerationData{}
	}
	klog.Infof("create notebook params:%v", message)
	if err := nc.notebookHandler.SubmitNotebookByData(*message); err != nil {
		log.Errorf("SubmitNotebook err : %s", err.Error())
		utils.Failed(c, err.Error())
		return
	}
	utils.Succeed(c, "Create success!")
}
