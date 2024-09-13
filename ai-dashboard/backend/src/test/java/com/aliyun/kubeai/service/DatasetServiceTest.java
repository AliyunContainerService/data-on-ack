package com.aliyun.kubeai.service;

import org.junit.Test;
import org.junit.runner.RunWith;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.junit4.SpringRunner;

import javax.annotation.Resource;

@RunWith(SpringRunner.class)
@SpringBootTest
public class DatasetServiceTest {

    @Resource
    private DatasetService datasetService;


    @Test
    public void deleteDataset() {
        //datasetService.deleteDataset(1L);
    }
}
