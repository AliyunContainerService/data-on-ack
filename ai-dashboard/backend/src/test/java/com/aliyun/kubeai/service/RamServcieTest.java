package com.aliyun.kubeai.service;

import com.aliyun.kubeai.cluster.AliyunClient;
import com.aliyun.kubeai.cluster.KubeClient;
import com.aliyun.kubeai.model.auth.RamUser;
import com.aliyun.kubeai.model.auth.RamWebApplication;
import com.google.gson.Gson;
import lombok.extern.slf4j.Slf4j;
import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ContextConfiguration;
import org.springframework.test.context.junit4.SpringJUnit4ClassRunner;

import java.util.Arrays;
import java.util.List;

@SpringBootTest
@RunWith(SpringJUnit4ClassRunner.class)
@ContextConfiguration(classes = {RamService.class, AliyunClient.class, KubeClient.class})
@Slf4j
public class RamServcieTest {
    private String webAppName;

    //@Resource
    @Autowired
    RamService ramService;

    @Before
    public void setUp() {
        webAppName = "testClusterID-kube-ai-webapp";
    }

    @After
    public void shutDown() {
        ramService.deleteWebApp(webAppName);
    }

    @Test
    public void testCreateWebapp() {
        try {
            RamWebApplication app = ramService.getWebAppByName(webAppName);
            log.info("get app res:{}", app);
            if (app == null) {
                app = ramService.createWebApp(webAppName, "http://dashboard.kubeai.com/login/aliyun", Arrays.asList("aliuid", "profile"));
                log.info("create app res:{}", app);
            }
        } catch (Exception e) {
            log.error("create app res exception:{}", e);
            return;
        }
    }

    @Test
    public void testListRamUsers() {
        try {
            List<RamUser> ramUsers = ramService.listRamUser();
            log.info("list ram users:{}", new Gson().toJson(ramUsers));
        } catch (Exception e) {
            log.error("list ram users test exception:", e);
        }
    }
}
