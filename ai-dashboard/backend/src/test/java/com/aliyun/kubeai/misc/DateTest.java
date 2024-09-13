package com.aliyun.kubeai.misc;

import com.aliyun.kubeai.utils.DateUtil;
import org.junit.Test;

import java.sql.Timestamp;
import java.text.SimpleDateFormat;
import java.time.Instant;
import java.util.Date;
import java.util.concurrent.TimeUnit;

public class DateTest {

    private SimpleDateFormat sdf = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss");

    @Test
    public void timestampToDate() {
        long createTimestamp = 1624270200;
        Timestamp stamp = new Timestamp(createTimestamp);
        Date date = new Date(stamp.getTime());
        System.out.println(date);

        Date time = new Date(TimeUnit.MILLISECONDS.convert(createTimestamp, TimeUnit.SECONDS));
        System.out.println(time);

        Instant instant = Instant.ofEpochSecond(createTimestamp);
        Date d = Date.from(instant);
        System.out.println(d);
    }

    @Test
    public void testCoreHour() {
        int gpu = 1;
        float cpuCore = 4.0f;
        long duration = 209;
        float hour = Float.valueOf(duration) / 3600;

        System.out.println(gpu * hour);
        System.out.println(cpuCore * hour);
    }

    @Test
    public void testStr() {
        String duration = "678s";
        long s = Long.valueOf(duration.substring(0, duration.length() - 1));
        System.out.println(s);
    }

    @Test
    public void testTimeZone() {
        Date now = new Date();
        String strtime = sdf.format(now);
        System.out.println(strtime);
    }

    @Test
    public void testTransUtcTime() {
        System.out.println(DateUtil.transUTCTime("2021-01-21T12:37:33Z"));
    }

    @Test
    public void testUserData() {
        String RAW_USER_DATA = "#!/bin/bash\n" +
                "mkdir -p /var/log/acs\n" +
                "curl http://aliacs-k8s-cn-beijing.oss-cn-beijing-internal.aliyuncs.com/public/pkg/run/attach/1.16.9-aliyun.1/attach_node.sh | bash -s -- --docker-version 19.03.5 --token l9ivur.fphwo4piq9jx9wsk --endpoint 192.168.0.228:6443 --cluster-dns 172.23.0.10 --node-name-mode nodeip --labels workload_type=cpu,policy=recycle,k8s.aliyun.com=true,%s --cms-enabled --cms-version 1.3.7 --openapi-token 4e4d4f584979574274597148385449368ce876a5114aea39f4921035f30fb83b --addon-names flannel,arms-prometheus,csi-plugin,csi-provisioner,logtail-ds,ack-node-problem-detector,nginx-ingress-controller,kube-flannel-ds --cpu-policy none --node-cidr-mask 26 --node-port-range 30000-32767 --runtime docker --runtime-version 19.03.5 --timezone Asia/Shanghai | tee /var/log/acs/init.log\n" +
                "set +e\n" +
                "\n" +
                "set -e";

        String str = String.format(RAW_USER_DATA, "workloadId=1213131");
        System.out.println(str);
    }

    @Test
    public void testYaml() {
        String INFERENCE_SERVICE_JSON = "{\n" +
                "    \"apiVersion\":\"serving.kubeflow.org/v1alpha2\",\n" +
                "    \"kind\":\"InferenceService\",\n" +
                "    \"metadata\":{\n" +
                "        \"name\":\"%s\"\n" +
                "    },\n" +
                "    \"spec\":{\n" +
                "        \"default\":{\n" +
                "            \"minReplicas\":\"%d\",\n" +
                "            \"maxReplicas\":\"%d\",\n" +
                "            \"parallelism\":\"%d\",\n" +
                "            \"predictor\":{\n" +
                "                \"tensorflow\":{\n" +
                "                    \"storageUri\":\"pvc://%s%s\",\n" +
                "                    \"runtimeVersion\":\"2.3.0-gpu\",\n" +
                "                    \"resources\":{\n" +
                "                        \"limits\":{\n" +
                "                            \"cpu\":\"4\",\n" +
                "                            \"memory\":\"8Gi\",\n" +
                "                            \"nvidia.com/gpu\":1\n" +
                "                        }\n" +
                "                    }\n" +
                "                }\n" +
                "            }\n" +
                "        }\n" +
                "    }\n" +
                "}";
        System.out.println(INFERENCE_SERVICE_JSON);
    }

    @Test
    public void testStr2() {
        String ctxName = "kubernetes-admin-c9522b942edaf4ec98c2cbc351ec8ade6";
        ctxName = ctxName.substring(ctxName.lastIndexOf("-") + 1);
        System.out.println(ctxName);
    }
}
