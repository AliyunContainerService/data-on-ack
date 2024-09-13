package com.aliyun.kubeai.misc;

import org.junit.Test;

import java.util.UUID;

public class StringTest {

    @Test
    public void testStr() {
        String workload = "seldon-12121213131";
        String str = workload.substring(workload.indexOf("-") + 1);
        System.out.println(str);
    }

    @Test
    public void testUUID() {
        String uuid = UUID.randomUUID().toString().replaceAll("-", "");
        System.out.println(uuid);
    }
}
