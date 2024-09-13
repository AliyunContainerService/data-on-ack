package com.aliyun.kubeai.service;

import com.aliyun.kubeai.cluster.AliyunClient;
import com.aliyun.kubeai.cluster.KubeClient;
import com.aliyun.kubeai.dao.K8sUserDao;
import com.aliyun.kubeai.mapper.JobInstanceMapper;
import com.aliyun.kubeai.mapper.ServingJobMapper;
import com.aliyun.kubeai.mapper.TrainingJobMapper;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.ComponentScan;
import org.springframework.context.annotation.Configuration;

@Configuration
@ComponentScan(basePackages = {"com.aliyun.kubeai.service"})
public class ServiceTestContext {
    @Bean
    KubeClient kubeClient() {
        return new KubeClient();
    }

    @Bean
    AliyunClient aliyunClient() {
        return new AliyunClient();
    }

    @Bean
    K8sUserDao k8sUserDao() {
        return new K8sUserDao();
    }

    @Bean
    TrainingJobMapper trainingJobMapper() {
        return null;
    }

    @Bean
    ServingJobMapper servingJobMapper() {
        return null;
    }

    @Bean
    JobInstanceMapper jobInstanceMapper() {
        return null;
    }
}
