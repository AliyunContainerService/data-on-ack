package com.aliyun.kubeai.misc;

import lombok.extern.slf4j.Slf4j;
import org.junit.Test;

@Slf4j
public class TestFloat {

    @Test
    public void testPrice() {
        int gpu = 4;
        int requestGpus = 1;

        float tradePrice = 0.021936f;
        float onDemandPrice = 0.021936f;
        log.info("======= tradePrice: {} onDemandPrice:{}", tradePrice, onDemandPrice);

        log.info("======= {}", (requestGpus / (float)gpu));

        if (requestGpus > 0) {
            tradePrice = tradePrice * (requestGpus / (float)gpu);
            onDemandPrice = onDemandPrice * (requestGpus / gpu);
        }
        log.info("======= tradePrice: {} onDemandPrice:{}", tradePrice, onDemandPrice);

    }

}
