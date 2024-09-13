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
    
package com.aliyun.kubeai.service;

import com.aliyun.kubeai.cluster.AliyunClient;
import com.aliyun.kubeai.model.auth.RamSecret;
import com.aliyun.kubeai.model.auth.RamUser;
import com.aliyun.kubeai.model.auth.RamWebApplication;
import com.aliyun.tea.TeaException;
import com.aliyun.tea.TeaUnretryableException;
import com.aliyun.tea.ValidateException;
import com.google.common.base.Strings;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.util.List;

@Slf4j
@Service
public class RamService {
    private static final String DisplayAppName = "kube-ai-dashboard";
    private static final String ENV_MY_POD_NAME = "MY_POD_NAME";

    @Resource
    private AliyunClient aliyunClient;

    public RamService() {
        if (aliyunClient == null) {
            aliyunClient = new AliyunClient();
        }
    }

    public List<RamUser> listRamUser() throws Exception{
        try {
            List<RamUser> userList = aliyunClient.listUser(null);
            return userList;
        } catch(ValidateException e) {
            log.error("list ram user ValidateException:", e);
            log.warn("list ram user ValidateException message:{}", e.getMessage());
            throw e;
        } catch (TeaUnretryableException e) {
            log.error("list ram user TeaUnretryableException:", e);
            log.warn("list ram user TeaUnretryableException message:{}", e.getMessage());
            log.warn("plist ram user TeaUnretryableException last request:{}", e.getLastRequest());
            throw e;
        } catch (TeaException e) {
            log.error("list ram user TeaException:", e);
            log.warn("list ram user TeaException code:{}", e.getCode());
            log.warn("list ram user TeaException message:{}", e.getMessage());
            log.warn("list ram user TeaException data:{}", e.getData());
            throw e;
        } catch (Exception e) {
            log.error("list ram user Exception:", e);
            throw e;
        }
    }

    public RamWebApplication getWebAppByName(String webAppName) throws Exception{
        try {
            RamWebApplication app = aliyunClient.getWebApp(null, webAppName);
            if (app != null) {
                RamSecret secret = aliyunClient.getAppSecret(app.getAppId(), true);
                app.setSecret(secret);
            }
            return app;
        } catch(ValidateException e) {
            log.error("delete web app ValidateException:", e);
            log.warn("delete web app ValidateException message:{}", e.getMessage());
            throw e;
        } catch (TeaUnretryableException e) {
            log.error("delete web app TeaUnretryableException:", e);
            log.warn("delete web app TeaUnretryableException message:{}", e.getMessage());
            log.warn("delete web app TeaUnretryableException last request:{}", e.getLastRequest());
            throw e;
        } catch (TeaException e) {
            log.error("delete web app TeaException:", e);
            log.warn("delete web app TeaException code:{}", e.getCode());
            log.warn("delete web app TeaException message:{}", e.getMessage());
            log.warn("delete web app TeaException data:{}", e.getData());
            throw e;
        } catch (Exception e) {
            log.error("delete web app Exception:", e);
            throw e;
        }
    }

    public void deleteWebApp(String appName) {
        try {
            RamWebApplication app = aliyunClient.getWebApp(null, appName);
            if (app == null || Strings.isNullOrEmpty(app.getAppId())) {
                log.info("delete app not found appName:{}", appName);
                return;
            }
            String myPodName = System.getenv(ENV_MY_POD_NAME);
            String podId = parsePodIdFromPodName();
            if (!Strings.isNullOrEmpty(podId)) {
                if (!parsePodIdFromDisplayName(app.getDisplayName()).equals(podId)){
                    log.info("pod id not match between podName:{} and displayName:{}", myPodName, app.getDisplayName());
                    return;
                }
            }
            aliyunClient.deleteWebApp(app.getAppId());
        } catch(ValidateException e) {
            log.error("delete web app ValidateException:", e);
            log.warn("delete web app ValidateException message:{}", e.getMessage());
        } catch (TeaUnretryableException e) {
            log.error("delete web app TeaUnretryableException:", e);
            log.warn("delete web app TeaUnretryableException message:{}", e.getMessage());
            log.warn("delete web app TeaUnretryableException last request:{}", e.getLastRequest());
        } catch (TeaException e) {
            log.error("delete web app TeaException:", e);
            log.warn("delete web app TeaException code:{}", e.getCode());
            log.warn("delete web app TeaException message:{}", e.getMessage());
            log.warn("delete web app TeaException data:{}", e.getData());
        } catch (Exception e) {
            log.error("delete web app exception:", e);
        }
        return;
    }

    private String parsePodIdFromPodName() {
        String myPodName = System.getenv(ENV_MY_POD_NAME);
        log.info("got podName:{}", myPodName);
        String podId = "";
        if (!Strings.isNullOrEmpty(myPodName)) {
            String[] podNameSplits = myPodName.split("-");
            //MY_POD_NAME=ack-ai-dashboard-admin-ui-855469c4-tgh8t
            if (podNameSplits.length > 0) {
                podId = podNameSplits[podNameSplits.length - 1] ;
            }
            log.info("got podId:{}", podId);
        }
        return podId;
    }

    private String parsePodIdFromDisplayName(String displayAppNameWithPodId) {
        String podId = "";
        String[] displayNameSplit = displayAppNameWithPodId.split("-");
        if (displayNameSplit.length > 0) {
            podId = displayNameSplit[displayNameSplit.length - 1];
        }
        return podId;
    }
    private String genDisplayAppName() {
        String podId = parsePodIdFromPodName();
        if (Strings.isNullOrEmpty(podId)) {
            return DisplayAppName;
        }
        return String.format("%s-%s", DisplayAppName, podId);
    }

    public RamWebApplication updateWebApp(RamWebApplication app, String redirectUri, List<String> preDefinedScopes) throws Exception{
        String displayNameWithPodId = genDisplayAppName();
        try {
            return aliyunClient.updateWebApp(app, displayNameWithPodId, redirectUri, preDefinedScopes);
        } catch(ValidateException e) {
            log.error("create web app ValidateException:", e);
            log.warn("create web app ValidateException message:{}", e.getMessage());
            throw e;
        } catch (TeaUnretryableException e) {
            log.error("create web app TeaUnretryableException:", e);
            log.warn("create web app TeaUnretryableException message:{}", e.getMessage());
            log.warn("create web app TeaUnretryableException last request:{}", e.getLastRequest());
            throw e;
        } catch (TeaException e) {
            log.error("create web app TeaException:", e);
            log.warn("create web app TeaException code:{}", e.getCode());
            log.warn("create web app TeaException message:{}", e.getMessage());
            log.warn("create web app TeaException data:{}", e.getData());
            throw e;
        } catch (Exception e) {
            log.error("create web app Exception:", e);
            throw e;
        }
    }

    public RamWebApplication createWebApp(String appName, String redirectUri, List<String> preDefinedScopes) throws Exception{
        RamWebApplication app = null;
        log.info("create web app by name:{} redirectUrl:{}", appName, redirectUri);
        try {
            String displayNameWithPodId = genDisplayAppName();
            app = aliyunClient.createWebApp(appName, displayNameWithPodId, redirectUri, preDefinedScopes);
            if (app!=null) {
                RamSecret secret = aliyunClient.getAppSecret(app.getAppId(), true);
                app.setSecret(secret);
            }
        } catch(ValidateException e) {
            log.error("create web app ValidateException:", e);
            log.warn("create web app ValidateException message:{}", e.getMessage());
            throw e;
        } catch (TeaUnretryableException e) {
            log.error("create web app TeaUnretryableException:", e);
            log.warn("create web app TeaUnretryableException message:{}", e.getMessage());
            log.warn("create web app TeaUnretryableException last request:{}", e.getLastRequest());
            throw e;
        } catch (TeaException e) {
            log.error("create web app TeaException:", e);
            log.warn("create web app TeaException code:{}", e.getCode());
            log.warn("create web app TeaException message:{}", e.getMessage());
            log.warn("create web app TeaException data:{}", e.getData());
            throw e;
        } catch (Exception e) {
            log.error("create web app Exception:", e);
            throw e;
        }
        return app;
    }
}
