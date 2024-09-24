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
	"encoding/json"
	"fmt"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/proxy"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"k8s.io/klog"
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

func (nc *NotebookAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	routes.POST("/notebook/create", nc.SubmitNotebook)
	routes.GET("/notebook/list", nc.GetNotebookList)
	//routes.GET("/notebook/stop", nc.DeleteNotebookByName)
	//routes.GET("/notebook/start", nc.DeleteNotebookByName)
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

func (nc *NotebookAPIsController) GetNotebookListFromStorage(c *gin.Context) {
	namespacesStr := c.Query("namespaces")
	userName := c.Query("userName")
	uid := c.Query("userId") //此处前端传过来的userId为uid
	var namespaces []string
	if err := json.Unmarshal([]byte(namespacesStr), &namespaces); err != nil {
		klog.Errorf("unmarshal namespace error:%s", err)
		utils.Failed(c, fmt.Sprintf("Namespace format error:%s", namespacesStr))
		return
	}
	namespacesMap := make(map[string]bool)
	for _, namespace := range namespaces {
		namespacesMap[namespace] = true
	}
	var resNotebookList []handlers.NotebookMessage
	if IsAdminUser(userName) || IsAdminUser(uid) {
		userName, uid = "", ""
	}
	for namespace, _ := range namespacesMap {
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
