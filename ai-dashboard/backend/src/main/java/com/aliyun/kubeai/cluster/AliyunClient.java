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
    
package com.aliyun.kubeai.cluster;

import com.alibaba.fastjson.JSON;
import com.alibaba.fastjson.JSONArray;
import com.alibaba.fastjson.JSONObject;
import com.aliyun.ims20190815.Client;
import com.aliyun.ims20190815.models.*;
import com.aliyun.kubeai.model.InstanceInfo;
import com.aliyun.kubeai.model.auth.AKInfo;
import com.aliyun.kubeai.model.auth.RamSecret;
import com.aliyun.kubeai.model.auth.RamUser;
import com.aliyun.kubeai.model.auth.RamWebApplication;
import com.aliyun.kubeai.utils.DateUtil;
import com.aliyun.kubeai.utils.HttpUtil;
import com.aliyun.teaopenapi.models.Config;
import com.aliyuncs.CommonRequest;
import com.aliyuncs.CommonResponse;
import com.aliyuncs.DefaultAcsClient;
import com.aliyuncs.IAcsClient;
import com.aliyuncs.eci.model.v20180808.DescribeContainerGroupsRequest;
import com.aliyuncs.ecs.model.v20140526.*;
import com.aliyuncs.exceptions.ClientException;
import com.aliyuncs.http.HttpResponse;
import com.aliyuncs.profile.DefaultProfile;
import com.google.common.base.Strings;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

import javax.annotation.PostConstruct;
import javax.annotation.Resource;
import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.ArrayList;
import java.util.Date;
import java.util.List;

@Slf4j
@Component
public class AliyunClient {
    private static final String DisplayAppName = "kube-ai-dashboard";
    private static final String ENV_MY_POD_NAME = "MY_POD_NAME";

    @Resource
    AuthUtil authUtil;

    @Resource
    MetadataClient metadataClient;

    private String regionId;
    private AKInfo akInfo;
    private IAcsClient acsClient;
    private com.aliyun.ims20190815.Client ramClient;


    @PostConstruct
    private void init() {
        if (Strings.isNullOrEmpty(regionId)) {
            try {
                regionId = metadataClient.getRegionId();
            } catch (Exception e) {
                log.error("get region error", e);
                regionId = null;
            }
        }

        buildOrRefreshAcsAndRamClient();
    }

    private IAcsClient newAcsClient(String regionId, AKInfo akinfo) {
        DefaultProfile profile = DefaultProfile.getProfile(regionId,
                akinfo.getAccessKeyId(),
                akinfo.getAccessKeySecret(),
                akinfo.getSecurityToken());

        return new DefaultAcsClient(profile);
    }

    private Client newRamClient(AKInfo akinfo) throws Exception {
        String ramDomain = "ims.vpc-proxy.aliyuncs.com";
        if (!HttpUtil.isDomainAvailable(ramDomain)) {
            ramDomain = "ims.aliyuncs.com";
        }
        log.info("using ram domain:{}", ramDomain);
        Config config = new Config().setAccessKeyId(akinfo.getAccessKeyId())
                .setAccessKeySecret(akinfo.getAccessKeySecret())
                .setEndpoint(ramDomain).setSecurityToken(akinfo.getSecurityToken());
        return new Client(config);
    }

    private void buildOrRefreshAcsAndRamClient() {
        try {
            akInfo = authUtil.getAKInfo();
            log.debug("akInfo: {}", JSON.toJSONString(akInfo));
            if(akInfo == null) {
                log.error("get ak info failed");
                return;
            }

            acsClient = newAcsClient(regionId, akInfo);
            ramClient = newRamClient(akInfo);
        } catch (Exception e) {
            log.error("build or refresh ack and ram client failed", e);
        }
    }

    private IAcsClient getAcsClient() {
        if (acsClient == null || isTokenExpired(akInfo)) {
            log.debug("refresh acs client");
            buildOrRefreshAcsAndRamClient();
        }
        return acsClient;
    }

    private Client getRamClient() throws Exception{
        if (ramClient == null || isTokenExpired(akInfo)) {
            buildOrRefreshAcsAndRamClient();
        }
        if (ramClient == null) {
            throw new Exception("build or refresh ram client failed");
        }
        return ramClient;
    }

    public List<RamUser> listUser(String userName) throws Exception{
        ListUsersRequest request = new ListUsersRequest();
        List<RamUser>  resUsers = new ArrayList<>();
        ListUsersResponse response = getRamClient().listUsers(request);
        for (ListUsersResponseBody.ListUsersResponseBodyUsersUser user : response.getBody().getUsers().getUser()) {
            RamUser ramUser = new RamUser();
            ramUser.setUserId(user.getUserId());
            ramUser.setUserName(user.getUserPrincipalName());
            ramUser.setDisplayName(user.getDisplayName());
            ramUser.setCreateDate(user.getCreateDate());
            ramUser.setUpdateDate(user.getUpdateDate());
            if (!Strings.isNullOrEmpty(userName) && user.getUserPrincipalName().equals(userName)) {
                resUsers.add(ramUser);
                continue;
            }
            resUsers.add(ramUser);
        }
        return resUsers;
    }

    public RamWebApplication updateWebApp(RamWebApplication app, String displayName, String redirectUri, List<String> preDefinedScopeList) throws Exception{
        String preDefinedScopes = String.join(";", preDefinedScopeList);
        log.info("update web app with id:{} name:{} displayName:{} redirectUri:{} preDefinedScope:{}",
                app.getAppId(), app.getAppName(),
                displayName, redirectUri, preDefinedScopes);
        Client client = getRamClient();
        UpdateApplicationRequest updateApplicationRequest = new UpdateApplicationRequest()
                .setAppId(app.getAppId())
                .setNewDisplayName(displayName)
                .setNewRedirectUris(redirectUri)
                .setNewPredefinedScopes(preDefinedScopes);
        UpdateApplicationResponse res = client.updateApplication(updateApplicationRequest);
        String requestId = res.getBody().getRequestId();
        log.info("update web app requestId:{} res:{}", requestId, res);
        UpdateApplicationResponseBody.UpdateApplicationResponseBodyApplication updatedApp = res.getBody().getApplication();
        RamWebApplication resApp = app;
        if (null != updatedApp) {
            String appStr = JSON.toJSONString(updatedApp);
            resApp = JSON.parseObject(appStr, RamWebApplication.class);
            resApp.setSecret(app.getSecret()); // copy secret from origin app
        }
        return resApp;
    }

    public RamWebApplication createWebApp(String appName, String displayName, String redirectUris, List<String> preDefinedScopeList) throws Exception {
        String preDefinedScopes = String.join(";", preDefinedScopeList);
        log.info("create web app with name:{} displayName:{} redirectUri:{} preDefinedScope:{}",
                appName, displayName, redirectUris, preDefinedScopes);
        RamWebApplication ret = null;
        CreateApplicationRequest createApplicationRequest = new CreateApplicationRequest()
                .setAppName(appName)
                .setDisplayName(displayName)
                .setAppType("WebApp")
                .setRedirectUris(redirectUris)
                .setSecretRequired(true)
                .setPredefinedScopes(preDefinedScopes);
        Client client = getRamClient();
        CreateApplicationResponse response = client.createApplication(createApplicationRequest);
        String requestId = response.getBody().getRequestId();
        CreateApplicationResponseBody.CreateApplicationResponseBodyApplication app = response.getBody().getApplication();
        if (app != null) {
            String appStr = JSON.toJSONString(app);
            log.info("create app ok, requestId:{} info:{}", requestId, appStr);
            ret = JSON.parseObject(appStr, RamWebApplication.class);
        }
        log.info("create web app ret:{}", ret);
        return ret;
    }

    public void deleteWebApp(String appId) throws Exception {
        if (Strings.isNullOrEmpty(appId)) {
            return;
        }
        DeleteApplicationRequest req = new DeleteApplicationRequest().setAppId(appId);
        DeleteApplicationResponse res = ramClient.deleteApplication(req);
        log.info("delete app ok appId:{} reqId:{}", appId, res.getBody().getRequestId());
        return;
    }

    public RamWebApplication getWebApp(String appId, String appName) throws Exception {
        Client client = getRamClient();
        RamWebApplication ret = null;
        if (appId == null && Strings.isNullOrEmpty(appName)) {
            return ret;
        }
        if (appId == null) {
            ListApplicationsRequest req = new ListApplicationsRequest();
            ListApplicationsResponseBody.ListApplicationsResponseBodyApplications res = client.listApplications(req).getBody().getApplications();
            log.info("get application res size:{}", res.getApplication().size());
            for (ListApplicationsResponseBody.ListApplicationsResponseBodyApplicationsApplication app : res.getApplication()) {
                if (appName.equals(app.getAppName())) {
                    log.info("found application by name:{} res:{}", appName, JSON.toJSONString(app));
                    ret = JSON.parseObject(JSON.toJSONString(app), RamWebApplication.class);
                    break;
                }
            }
        } else {
            GetApplicationRequest req = new GetApplicationRequest().setAppId(appId);
            GetApplicationResponseBody.GetApplicationResponseBodyApplication res = client.getApplication(req).getBody().getApplication();
            log.info("found application by id:{} res:{}", appId, JSON.toJSONString(res));
            if (res != null) {
                ret = JSON.parseObject(JSON.toJSONString(res), RamWebApplication.class);
            }
        }
        log.info("found application:{}", ret);

        return ret;
    }

    public RamSecret getAppSecret(String appId, boolean createIfNotExist) throws Exception {
        Client client = getRamClient();
        RamSecret ret = null;
        //set secret id and value
        if (Strings.isNullOrEmpty(appId)) {
            return ret;
        }
        ListAppSecretIdsRequest listSecretReq = new ListAppSecretIdsRequest().setAppId(appId);
        ListAppSecretIdsResponseBody.ListAppSecretIdsResponseBodyAppSecrets appSecrets = client.listAppSecretIds(listSecretReq).getBody().getAppSecrets();
        String secretId = null;
        String secretValue = null;
        if (appSecrets == null || appSecrets.getAppSecret().isEmpty()) {
            log.info("create app secret createIfNotExist:{}", createIfNotExist);
            if (createIfNotExist) {
                CreateAppSecretRequest req = new CreateAppSecretRequest().setAppId(appId);
                CreateAppSecretResponseBody.CreateAppSecretResponseBodyAppSecret res = client.createAppSecret(req).getBody().getAppSecret();
                ret = JSON.parseObject(JSON.toJSONString(res), RamSecret.class);
                //secretId = res.getAppSecretId();
                //secretValue = res.getAppSecretValue();
            }
        } else {
            List<ListAppSecretIdsResponseBody.ListAppSecretIdsResponseBodyAppSecretsAppSecret> secrets = appSecrets.getAppSecret();
            secretId = secrets.get(0).getAppSecretId();
            log.info("found app secret len:{} secrets:{}", secrets.size(), JSON.toJSONString(secrets));
            GetAppSecretRequest getSecretReq = new GetAppSecretRequest().setAppId(appId).setAppSecretId(secretId);
            GetAppSecretResponse getSecretRes = client.getAppSecret(getSecretReq);
            ret = JSON.parseObject(JSON.toJSONString(getSecretRes.getBody().getAppSecret()), RamSecret.class);
        }
        log.info("got ram secret:{}", ret);
        return ret;
    }

    public float getEciPrice(String instanceType) {
        log.debug("get eci price, instanceType:{}", instanceType);
        CommonRequest request = new CommonRequest();
        request.setSysDomain("eci.aliyuncs.com");
        request.setSysRegionId("cn-beijing");
        request.setSysVersion("2018-08-08");
        request.setSysAction("DescribeContainerGroupPrice");

        request.putQueryParameter("RegionId", "cn-beijing");
        request.putQueryParameter("InstanceType", instanceType);

        try {
            CommonResponse response = getAcsClient().getCommonResponse(request);
            JSONObject rootNode = JSON.parseObject(response.getData());
            JSONObject priceNode = rootNode.getJSONObject("PriceInfo").getJSONObject("Price");
            float originalPrice = priceNode.getFloat("OriginalPrice");
            float secondPrice = originalPrice / 3600;
            secondPrice = formatFloat(secondPrice);
            log.debug("eci price, instance:{} originalPrice:{} secondPrice:{}", instanceType, originalPrice, secondPrice);
            return secondPrice;
        } catch (ClientException e) {
            log.error("get eci price failed", e);
        }
        return 0f;
    }

    public float getSpotPrice(String instanceType) {
        log.debug("get spot price, instanceType:{}", instanceType);
        DescribeSpotPriceHistoryRequest request = new DescribeSpotPriceHistoryRequest();
        request.setInstanceType(instanceType);
        request.setNetworkType("vpc");
        request.setZoneId("cn-beijing-h");

        try {
            DescribeSpotPriceHistoryResponse response = getAcsClient().getAcsResponse(request);
            List<DescribeSpotPriceHistoryResponse.SpotPriceType> spotPrices = response.getSpotPrices();
            int num = spotPrices.size();
            float total = 0f;
            for (DescribeSpotPriceHistoryResponse.SpotPriceType spotPriceType : spotPrices) {
                total += spotPriceType.getSpotPrice();
            }
            //每秒价格
            float avgSecondPrice = total / (num * 3600);
            avgSecondPrice = formatFloat(avgSecondPrice);
            log.debug("spot price, instanceType:{} secondPrice:{}", instanceType, BigDecimal.valueOf(avgSecondPrice));
            return avgSecondPrice;
        } catch (ClientException e) {
            log.error("get spot price failed", e);
        }
        return 0f;
    }

    public float getEcsPrice(String instanceType) {
        log.debug("get ecs price, instanceType:{}", instanceType);

        DescribePriceRequest request = new DescribePriceRequest();
        request.setInstanceType(instanceType);

        try {
            DescribePriceResponse response = getAcsClient().getAcsResponse(request);
            float originalPrice = response.getPriceInfo().getPrice().getOriginalPrice();
            //每秒价格
            float secondPrice = originalPrice / 3600;
            secondPrice = formatFloat(secondPrice);
            log.debug("ecs price, instanceType:{} originalPrice:{} secondPrice:{}", instanceType, originalPrice, secondPrice);
            return secondPrice;
        } catch (ClientException e) {
            log.error("get ecs price failed", e);
        }
        return 0f;
    }

    @Deprecated
    public String getEciInstanceType(String eciName) {
        log.debug("get eci instance type, eciName:{}", eciName);
        DescribeContainerGroupsRequest request = new DescribeContainerGroupsRequest();
        request.setContainerGroupName(eciName);

        try {
            HttpResponse response = getAcsClient().doAction(request);
            String body = new String(response.getHttpContent());
            JSONObject rootNode = JSON.parseObject(body);
            JSONArray arr = rootNode.getJSONArray("ContainerGroups");
            JSONObject eciNode = arr.getJSONObject(0);
            String instanceType = eciNode.getString("InstanceType");
            return instanceType;
        } catch (ClientException e) {
            log.error("get eci instance failed}", e);
        }
        return null;
    }

    public InstanceInfo getEciInstance(String eciName) {
        log.debug("get eci instance, eciName:{}", eciName);
        DescribeContainerGroupsRequest request = new DescribeContainerGroupsRequest();
        request.setContainerGroupName(eciName);

        try {
            HttpResponse response = getAcsClient().doAction(request);
            String body = new String(response.getHttpContent());
            JSONObject rootNode = JSON.parseObject(body);
            JSONArray arr = rootNode.getJSONArray("ContainerGroups");
            JSONObject eciNode = arr.getJSONObject(0);
            String instanceType = eciNode.getString("InstanceType");
            float cpuCore = eciNode.getFloat("Cpu");
            int gpu = eciNode.getInteger("Gpu");

            InstanceInfo instanceInfo = new InstanceInfo();
            instanceInfo.setInstanceType(instanceType);
            instanceInfo.setCpuCore(cpuCore);
            instanceInfo.setGpu(gpu);
            return instanceInfo;
        } catch (ClientException e) {
            log.error("get eci instance failed}", e);
        }
        return null;
    }

    public InstanceInfo getEcsInstance(String nodeName, String nodeIp) {
        log.info("get ecs instance, nodeName:{} nodeIp:{}", nodeName, nodeIp);
        JSONArray arr = new JSONArray();
        arr.add(nodeIp);

        String instanceType = "";
        String resourceType = "ECS";
        float cpuCore;
        int gpu;
        boolean isSpot = false;

        DescribeInstancesRequest request = new DescribeInstancesRequest();
        request.setInstanceNetworkType("vpc");
        request.setPrivateIpAddresses(arr.toJSONString());
        try {
            DescribeInstancesResponse response = getAcsClient().getAcsResponse(request);
            if (response.getInstances() == null || response.getInstances().isEmpty()) {
                log.error("get ecs instance type failed: asc response empty, result:{}", JSON.toJSONString(response));
                return null;
            }
            cpuCore = Float.valueOf(response.getInstances().get(0).getCpu());
            gpu = response.getInstances().get(0).getGPUAmount();
            instanceType = response.getInstances().get(0).getInstanceType();
            String spotStrategy = response.getInstances().get(0).getSpotStrategy();
            isSpot = !spotStrategy.equals("NoSpot");
        } catch (ClientException e) {
            log.error("get ecs instance type failed", e);
            return null;
        }

        float onDemandPrice = getEcsPrice(instanceType);
        float tradePrice;
        if (isSpot) {
            tradePrice = getSpotPrice(instanceType);
        } else {
            tradePrice = onDemandPrice;
        }

        InstanceInfo instanceInfo = new InstanceInfo();
        instanceInfo.setInstanceType(instanceType);
        instanceInfo.setResourceType(resourceType);
        instanceInfo.setSpot(isSpot);
        instanceInfo.setOnDemandPrice(onDemandPrice);
        instanceInfo.setTradePrice(tradePrice);
        instanceInfo.setCpuCore(cpuCore);
        instanceInfo.setGpu(gpu);
        return instanceInfo;
    }

    private float formatFloat(float val) {
        BigDecimal b = new BigDecimal(val);
        return b.setScale(6, RoundingMode.HALF_UP).floatValue();
    }

    private boolean isTokenExpired(AKInfo info) {
        Date expiredTime = DateUtil.getDateFromUTC(info.getExpiration());
        Date now = new Date();
        return now.getTime() >= expiredTime.getTime();
    }
}
