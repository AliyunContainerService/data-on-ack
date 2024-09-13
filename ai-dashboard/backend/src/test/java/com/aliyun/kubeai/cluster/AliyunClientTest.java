package com.aliyun.kubeai.cluster;

import com.aliyun.kubeai.model.auth.RamWebApplication;
import lombok.extern.slf4j.Slf4j;
import org.junit.Before;
import org.junit.Test;

import java.math.BigDecimal;
import java.util.Arrays;

@Slf4j
public class AliyunClientTest {

    String instanceType = "ecs.gn6i-c4g1.xlarge";

    private AliyunClient aliyunClient;

    @Before
    public void setUp() {
        aliyunClient = new AliyunClient();
    }

    @Test
    public void testGetPrice() {
        float ecsPrice = aliyunClient.getEcsPrice(instanceType);
        float eciPrice = aliyunClient.getEciPrice(instanceType);
        float spotPrice = aliyunClient.getSpotPrice(instanceType);
        System.out.println("ecs:" + ecsPrice);
        System.out.println("eci:" + eciPrice);
        System.out.println("spot:" + BigDecimal.valueOf(spotPrice));
    }

    @Test
    public void testCreateWebApp() {
        try {
            String webAppName = "kube-ai-webapp";
            RamWebApplication app = aliyunClient.getWebApp(null, webAppName);
            if (app == null) {
                app = aliyunClient.createWebApp("kube-ai-webapp", "kube-ai-webapp", "http://dashboard.kubeai.com/login/aliyun", Arrays.asList("aliuid", "profile"));
                log.info("create app test ok:{}", app);
            } else {
                log.info("found app ok:{}", app);
            }
        } catch (Exception e) {
            log.error("create app test exception:", e);
        }
    }
}
