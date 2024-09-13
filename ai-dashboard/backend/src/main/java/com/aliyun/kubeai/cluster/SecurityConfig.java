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

import com.aliyun.kubeai.model.auth.RamWebApplication;
import com.aliyun.kubeai.service.RamService;
import com.aliyun.kubeai.utils.HttpUtil;
import com.google.common.base.Strings;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.security.oauth2.resource.ResourceServerProperties;
import org.springframework.boot.autoconfigure.security.oauth2.resource.UserInfoTokenServices;
import org.springframework.boot.web.servlet.FilterRegistrationBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.annotation.web.configuration.WebSecurityConfigurerAdapter;
import org.springframework.security.oauth2.client.OAuth2ClientContext;
import org.springframework.security.oauth2.client.OAuth2RestTemplate;
import org.springframework.security.oauth2.client.filter.OAuth2ClientAuthenticationProcessingFilter;
import org.springframework.security.oauth2.client.filter.OAuth2ClientContextFilter;
import org.springframework.security.oauth2.client.token.grant.code.AuthorizationCodeResourceDetails;
import org.springframework.security.oauth2.common.AuthenticationScheme;
import org.springframework.security.web.authentication.LoginUrlAuthenticationEntryPoint;
import org.springframework.security.web.authentication.www.BasicAuthenticationFilter;

import javax.annotation.PreDestroy;
import javax.annotation.Resource;
import javax.servlet.Filter;
import java.util.Arrays;
import java.util.List;
import java.util.concurrent.TimeUnit;

@Slf4j
@Configuration
public class SecurityConfig extends WebSecurityConfigurerAdapter  {

    private static final String ENV_DASHBOARD_HOST = "DASHBOARD_HOST";
    private static final String ENV_DASHBOARD_INGRESS_ENABLE = "DASHBOARD_INGRESS_ENABLE";
    private static final String DASHBOARD_FULLNAME = "ack-ai-dashboard-admin-ui";
    private static final String DASHBOARD_NAMESPACE = "kube-ai";
    private static final String DEFAULT_FILTER_PROCESSES_URL = "/login/aliyun";
    private String webAppName = null;

    @Value("${oauth.is-closing:false}")
    private boolean isTesting;

    @Resource
    private RamService ramService;

    @Resource
    private KubeClient kubeClient;

    @Autowired
    private OAuth2ClientContext oauth2ClientContext;

    @Bean
    public FilterRegistrationBean oauth2ClientFilterRegistration(
            OAuth2ClientContextFilter filter) {
        FilterRegistrationBean registration = new FilterRegistrationBean();
        registration.setFilter(filter);
        registration.setOrder(-100);
        return registration;
    }

    @PreDestroy
    public void destroy() {
        log.info("destroy app:{}", webAppName);
        if (Strings.isNullOrEmpty(webAppName)) {
            return;
        }
        ramService.deleteWebApp(webAppName);
    }

    @Override
    protected void configure(HttpSecurity http) throws Exception {
        log.info("is closing oauth:{}", isTesting);
        if (isTesting) {
            // @formatter:off
            http.antMatcher("/**").authorizeRequests().antMatchers("/**").permitAll()
                    .anyRequest().authenticated()
                    .and().exceptionHandling().authenticationEntryPoint(new LoginUrlAuthenticationEntryPoint("/"))
                    .and().csrf().disable();
            // @formatter:on
            return;
        }
        // @formatter:off
        http.antMatcher("/**").authorizeRequests().antMatchers("/", "/login/aliyun**",
                        "/favicon**", "/grafana/**", "/health",
                        "/fonts/**", "**/favicon**", "/images/**", "/js/**", "/css/**", "/static/**", "/user/login**",
                        "/login**", "/webjars/**").permitAll()
                .anyRequest().authenticated()
                .and().exceptionHandling().authenticationEntryPoint(new LoginUrlAuthenticationEntryPoint("/"))
                .and().logout().invalidateHttpSession(true).clearAuthentication(true).logoutSuccessUrl("/login").permitAll()
                .and().csrf().disable()
                .addFilterBefore(ssoFilter(), BasicAuthenticationFilter.class)
                .headers().frameOptions().sameOrigin();
        // @formatter:on
    }

    private Filter ssoFilter() {
        try {
            OAuth2ClientAuthenticationProcessingFilter aliyunFilter = new OAuth2ClientAuthenticationProcessingFilter(
                    DEFAULT_FILTER_PROCESSES_URL);
            AuthorizationCodeResourceDetails aliyunAuthorizationConfigDetails = aliyun();
            OAuth2RestTemplate facebookTemplate = new OAuth2RestTemplate(aliyunAuthorizationConfigDetails, oauth2ClientContext);
            aliyunFilter.setRestTemplate(facebookTemplate);
            UserInfoTokenServices tokenServices = new UserInfoTokenServices(aliyunResource().getUserInfoUri(),
                    aliyunAuthorizationConfigDetails.getClientId());
            tokenServices.setRestTemplate(facebookTemplate);
            aliyunFilter.setTokenServices(
                    new UserInfoTokenServices(aliyunResource().getUserInfoUri(), aliyunAuthorizationConfigDetails.getClientId()));
            return aliyunFilter;
        } catch (Exception e) {
            log.error("add ssoFilter exception", e);
        }
        return null;
    }

    public AuthorizationCodeResourceDetails aliyun() throws Exception{
        AuthorizationCodeResourceDetails res = new AuthorizationCodeResourceDetails();
        RamWebApplication app = ramService.getWebAppByName(genAppName());
        String redirectUri = genRedirectUri(3, 3);
        List<String> preDefinedScopes = Arrays.asList("aliuid", "profile");
        if (app == null ) {
            log.info("create app");
            app = ramService.createWebApp(webAppName, redirectUri, preDefinedScopes);
            if (app == null) {
                throw new Exception("create web app failed");
            }
        } else {
            app = ramService.updateWebApp(app, redirectUri, preDefinedScopes);
        }
        log.info("login with app:{}", app);
        String clientId = app.getAppId();
        String clientSecret = app.getSecret().getSecretValue();
        if (Strings.isNullOrEmpty(clientId) || Strings.isNullOrEmpty(clientSecret)) {
            throw new Exception("client id or secret empty");
        }
        log.info("got client id:{} and secret:{}", clientId, clientSecret);
        res.setClientId(clientId);
        res.setClientSecret(clientSecret);
        String ramDomain = getRamDomain(isIntlAccount());
        res.setAccessTokenUri(String.format("https://%s/v1/token", ramDomain));
        res.setUserAuthorizationUri(isIntlAccount()?"https://signin.alibabacloud.com/oauth2/v1/auth":"https://signin.aliyun.com/oauth2/v1/auth");
        res.setTokenName("access_token");
        res.setAuthenticationScheme(AuthenticationScheme.query);
        res.setClientAuthenticationScheme(AuthenticationScheme.form);

        return res;
    }

    private String genAppName() {
        String clusterId = kubeClient.getClusterId();
        webAppName = String.format("%s-kube-ai-dashboard", clusterId);
        return webAppName;
    }

    private boolean isIntlAccount() {
        return Boolean.parseBoolean(System.getenv("INTL_ACCOUNT"));
    }

    private String getRamDomain(boolean isIntl) {
        String oauthDomain = "oauth.vpc-proxy.aliyuncs.com";
        if (isIntl) {
            oauthDomain = "oauth-intl.vpc-proxy.aliyuncs.com";
        }

        if (!HttpUtil.isDomainAvailable(oauthDomain)) {
            oauthDomain = "oauth.aliyun.com";
            if (isIntl) {
                oauthDomain = "oauth.alibabacloud.com";
            }
        }
        log.info("using ram domain:{} isIntl:{}", oauthDomain, isIntl);
        return oauthDomain;
    }

    public ResourceServerProperties aliyunResource() {
        ResourceServerProperties res = new ResourceServerProperties();
        String ramDomain = getRamDomain(isIntlAccount());
        res.setUserInfoUri(String.format("https://%s/v1/userinfo", ramDomain));
        return res;
    }

    private String genRedirectUri(int retryTimes, int retryIntervalSec) throws Exception{
        String envEnableIngress = System.getenv(ENV_DASHBOARD_INGRESS_ENABLE);
        if (Strings.isNullOrEmpty(envEnableIngress)) {
            return System.getenv(ENV_DASHBOARD_HOST); // backward compatible
        }
        Boolean isIngressEnabled = Boolean.parseBoolean(envEnableIngress);
        String dashboardHost = null;
        int i = 0;
        while (i < retryTimes) {
            if (isIngressEnabled) {
                dashboardHost = kubeClient.getIngressHostByName(DASHBOARD_FULLNAME, DASHBOARD_NAMESPACE);
            } else {
                dashboardHost = kubeClient.getClusterIpServiceByName(DASHBOARD_FULLNAME, DASHBOARD_NAMESPACE);
            }
            if (Strings.isNullOrEmpty(dashboardHost)){
                TimeUnit.SECONDS.sleep(retryIntervalSec);
                i++;
            } else {
                break;
            }
        }
        String res = String.format("http://%s%s", dashboardHost, DEFAULT_FILTER_PROCESSES_URL);
        log.info("redirect uri:{}", res);
        return res;
    }
}
