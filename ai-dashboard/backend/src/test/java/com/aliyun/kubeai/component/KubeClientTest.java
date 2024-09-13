package com.aliyun.kubeai.component;

import com.aliyun.kubeai.cluster.KubeClient;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.junit4.SpringRunner;

import javax.annotation.Resource;
import static org.junit.Assert.assertEquals;


@RunWith(SpringRunner.class)
@SpringBootTest
public class KubeClientTest {
    @Resource
    KubeClient client;

    @Test
    public void getIngress() {
        String ingressHost = client.getIngressHostByName("ack-ai-dev-console", "kube-ai");
        String targetHost = "ai-dev.c6b6dd183df6b4ddb96a3719719211a2f.cn-zhangjiakou.alicontainer.com";
        assertEquals(ingressHost, targetHost);
    }

    @Test
    public void getService() {
        String got = client.getClusterIpServiceByName("ack-ai-dev-console", "kube-ai");
        String want = "192.168.180.131";
        assertEquals(got, want);
    }
}
