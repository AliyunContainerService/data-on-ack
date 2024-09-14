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
	"encoding/json"
	"errors"
	"fmt"
	cli "github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/client"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/model"
	ims "github.com/alibabacloud-go/ims-20190815/v2/client"
	"k8s.io/klog"
	"os"
	"strings"
)

const (
	defaultAppName = "kube-ai-dev-console"
	envMyPodName   = "MY_POD_NAME"
)

type RedirectUris struct {
	RedirectUri []*string `json:"RedirectUri,omitempty" xml:"RedirectUri,omitempty" type:"Repeated"`
}

type Application struct {
	AkProxySuffix       string `json:"AkProxySuffix,omitempty" xml:"AkProxySuffix,omitempty"`
	DisplayName         string `json:"DisplayName,omitempty" xml:"DisplayName,omitempty"`
	AccessTokenValidity int32  `json:"AccessTokenValidity,omitempty" xml:"AccessTokenValidity,omitempty"`
	SecretRequired      bool   `json:"SecretRequired,omitempty" xml:"SecretRequired,omitempty"`
	AccountId           string `json:"AccountId,omitempty" xml:"AccountId,omitempty"`
	CreateDate          string `json:"CreateDate,omitempty" xml:"CreateDate,omitempty"`
	AppName             string `json:"AppName,omitempty" xml:"AppName,omitempty"`
	//RedirectUris         string `json:"RedirectUris,omitempty" xml:"RedirectUris,omitempty" type:"Struct"`
	UpdateDate string `json:"UpdateDate,omitempty" xml:"UpdateDate,omitempty"`
	//DelegatedScope       string `json:"DelegatedScope,omitempty" xml:"DelegatedScope,omitempty" type:"Struct"`
	PredefinedScopes     string `json:"PredefinedScopes,omitempty" xml:"PredefinedScopes,omitempty"`
	AppId                string `json:"AppId,omitempty" xml:"AppId,omitempty"`
	RefreshTokenValidity int32  `json:"RefreshTokenValidity,omitempty" xml:"RefreshTokenValidity,omitempty"`
	IsMultiTenant        bool   `json:"IsMultiTenant,omitempty" xml:"IsMultiTenant,omitempty"`
	AppType              string `json:"AppType,omitempty" xml:"AppType,omitempty"`
}

type AppSecret struct {
	AppSecretValue string `json:"AppSecretValue,omitempty" xml:"AppSecretValue,omitempty"`
	AppId          string `json:"AppId,omitempty" xml:"AppId,omitempty"`
	AppSecretId    string `json:"AppSecretId,omitempty" xml:"AppSecretId,omitempty"`
	CreateDate     string `json:"CreateDate,omitempty" xml:"CreateDate,omitempty"`
}

func GetDefaultDomain(client *ims.Client) (string, error) {
	if client == nil {
		return "", errors.New("ram client empty")
	}
	request := &ims.GetDefaultDomainRequest{}
	response, err := client.GetDefaultDomain(request)
	if err != nil {
		return "", err
	}
	return *response.Body.DefaultDomainName, nil
}

func GetOrCreateApplicationSecret(client *ims.Client, appID string) (*AppSecret, error) {
	listRequest := &ims.ListAppSecretIdsRequest{}
	listRequest.SetAppId(appID)
	listResponse, listErr := client.ListAppSecretIds(listRequest)
	if listErr != nil {
		return nil, listErr
	}

	resSecret := &AppSecret{}
	if len(listResponse.Body.AppSecrets.AppSecret) > 0 {
		getRequest := &ims.GetAppSecretRequest{}
		getRequest.SetAppId(appID)
		getRequest.SetAppSecretId(*listResponse.Body.AppSecrets.AppSecret[0].AppSecretId)
		getResponse, getErr := client.GetAppSecret(getRequest)
		if getErr != nil {
			return nil, getErr
		}
		err := deserializeImsObject(getResponse.Body.AppSecret, resSecret)
		return resSecret, err
	}

	// Create application secret if not exists
	createRequest := &ims.CreateAppSecretRequest{}
	createRequest.SetAppId(appID)
	createResponse, createErr := client.CreateAppSecret(createRequest)
	if createErr != nil {
		return nil, createErr
	}
	err := deserializeImsObject(createResponse.Body.AppSecret, resSecret)
	return resSecret, err
}

func parsePodIdFromDisplayName(displayName string) string {
	return parsePodIdFromPodName(displayName)
}

func parsePodIdFromPodName(podName string) string {
	if "" == podName {
		return ""
	}
	//MY_POD_NAME=ack-ai-dashboard-admin-ui-855469c4-tgh8t
	podNameSplited := strings.Split(podName, "-")
	if len(podNameSplited) > 0 {
		return podNameSplited[len(podNameSplited)-1]
	}
	return ""
}

func genDisplayName(appName string) string {
	myPodName := os.Getenv(envMyPodName)
	podId := parsePodIdFromPodName(myPodName)
	if "" == podId {
		return appName
	}
	return fmt.Sprintf("%s-%s", appName, podId)
}

func GenOAuthApp(ramClient *ims.Client) (*model.OAuthApp, error) {
	//clusterID := "c3292d86396634a7cbc9abc3eafc63742"
	//regionID := "cz-shanghai"
	//namespace := "kube-dl"
	//defaultDomain := "default.domain"
	//return model.OAuthApp{
	//	Name: clusterID + "-" + namespace + "-oauth-app",
	//	Context: map[string]string{
	//		model.ClusterID:     clusterID,
	//		model.RegionID:      regionID,
	//		model.AppNamespace:  "kubedl",
	//		model.DefaultDomain: defaultDomain,
	//	},
	//}, nil
	clusterID := cli.GetClusterId()
	regionID, err := cli.GetAliyunRamClient().GetMetadataClient().GetRegionId()
	if err != nil {
		return nil, err
	}
	defaultDomain, _ := GetDefaultDomain(ramClient)
	return &model.OAuthApp{
		Name:        defaultAppName,
		DisplayName: genDisplayName(defaultAppName),
		Context: map[string]string{
			model.ClusterID:     clusterID,
			model.RegionID:      regionID,
			model.AppNamespace:  "kube-ai",
			model.AppName:       "ai-dev",
			model.DefaultDomain: defaultDomain,
		},
	}, nil
}

func GetOrCreateApplication(client *ims.Client, oAuthApp *model.OAuthApp) (*Application, error) {
	if client == nil {
		return nil, errors.New("ram client empty")
	}
	if oAuthApp == nil {
		return nil, errors.New("oauth app config empty")
	}
	app, err := getApplication(client, oAuthApp)
	if err != nil {
		klog.Errorf("get app error")
		return nil, err
	}
	if nil != app {
		// update app for redirect uri
		if err = updateApplication(client, app, oAuthApp); err != nil {
			return app, err
		}
		return app, nil
	}
	return createApplication(client, oAuthApp)
}

func DeleteAppDefer() {
	klog.Infof("to delete webapp")

	ramClient, err := cli.GetAliyunRamClient().GetRamClient()
	if err != nil {
		klog.Errorf("to delete web app failed get ram client failed")
		return
	}
	oauthApplicationConfig := GetOauthAppConfig()
	if oauthApplicationConfig == nil {
		klog.Errorf("to delete web app failed get oauth config failed")
		return
	}
	if err = DeleteApplication(ramClient, oauthApplicationConfig); err != nil {
		klog.Errorf("delete web app error:%v", err)
		return
	}
	klog.Infof("delete web app success")
}

func DeleteApplication(client *ims.Client, oAuthApp *model.OAuthApp) error {
	if client == nil {
		return errors.New("ram client empty")
	}
	if oAuthApp == nil {
		return errors.New("oauth app config empty")
	}
	// Find oauth application id to delete
	app, err := getApplication(client, oAuthApp)
	if err != nil {
		klog.Errorf("find application to delete error %v", err)
		return err
	}
	if app == nil {
		klog.Infof("OAuthApp %s doesn't exists, no need to delete it\n", oAuthApp.GetAppName())
		return nil
	}
	podName := os.Getenv(envMyPodName)
	if "" != podName && parsePodIdFromDisplayName(app.DisplayName) != parsePodIdFromPodName(podName) {
		klog.Infof("pod id not match between podName:%s and displayName:%s", podName, app.DisplayName)
		return nil
	}
	deleteRequest := &ims.DeleteApplicationRequest{}
	deleteRequest.SetAppId(app.AppId)
	deleteResponse, deleteErr := client.DeleteApplication(deleteRequest)
	if deleteErr != nil {
		klog.Errorf("delete application failed, http status: %v, http response: %v",
			deleteResponse.Headers, deleteResponse.Body)
		return deleteErr
	}
	return nil
}

func deserializeImsObject(imsApplication interface{}, resObj interface{}) error {
	listAppJson, err := json.Marshal(imsApplication)
	if err != nil {
		return err
	}
	err = json.Unmarshal(listAppJson, resObj)
	if err != nil {
		return err
	}
	return nil
}

func getApplication(client *ims.Client, oAuthApp *model.OAuthApp) (resApp *Application, err error) {
	listRequest := &ims.ListApplicationsRequest{}
	listResponse, listErr := client.ListApplications(listRequest)
	if listErr != nil {
		klog.Errorf("list application failed %s", listErr)
		return nil, listErr
	}

	// Find oauth application
	for _, application := range listResponse.Body.Applications.Application {
		if *application.AppName != oAuthApp.GetAppName() {
			continue
		}
		resApp = &Application{}
		err = deserializeImsObject(application, resApp)
		if err != nil {
			klog.Errorf("get application deserialize failed %v", err)
			return nil, err
		}
		return resApp, nil
	}
	return nil, nil
}

func updateApplication(client *ims.Client, app *Application, oAuthApp *model.OAuthApp) error {
	// Create an new oauth application if not found
	redirectUri := oAuthApp.GetRedirectURI()
	displayName := genDisplayName(defaultAppName)
	updateRequest := &ims.UpdateApplicationRequest{}
	updateRequest.SetAppId(app.AppId)
	updateRequest.SetNewDisplayName(displayName)
	updateRequest.SetNewRedirectUris(redirectUri)
	klog.Infof("update app id:%s with redirect:%s displayName:%s", app.AppId, redirectUri, displayName)
	updateResponse, err := client.UpdateApplication(updateRequest)
	if err != nil {
		requestId := ""
		if nil != updateResponse && nil != updateResponse.Body && nil != updateResponse.Body.RequestId {
			requestId = *updateResponse.Body.RequestId
		}

		klog.Errorf("update application failed %s reqId:%s err:%s", oAuthApp.GetAppName(), requestId, err.Error())
		return err
	}
	return nil
}

func createApplication(client *ims.Client, oAuthApp *model.OAuthApp) (*Application, error) {
	// Create an new oauth application if not found
	redirectUri := oAuthApp.GetRedirectURI()
	createRequest := &ims.CreateApplicationRequest{}
	createRequest.SetAppName(oAuthApp.GetAppName())
	createRequest.SetAppType("WebApp")
	createRequest.SetDisplayName(oAuthApp.GetDisplayName())
	createRequest.SetRedirectUris(redirectUri)
	createRequest.SetPredefinedScopes("openid;aliuid;profile")
	klog.Infof("create app with redirect:%s", redirectUri)
	createResponse, createErr := client.CreateApplication(createRequest)
	if createErr != nil {
		klog.Errorf("create application failed %s", oAuthApp.GetAppName())
		return nil, createErr
	}
	resApp := &Application{}
	err := deserializeImsObject(createResponse.Body.Application, resApp)
	return resApp, err
}
