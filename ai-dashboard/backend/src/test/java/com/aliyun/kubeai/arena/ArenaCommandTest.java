package com.aliyun.kubeai.arena;

import lombok.extern.slf4j.Slf4j;
import org.apache.commons.lang3.StringUtils;
import org.junit.Test;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.util.ArrayList;
import java.util.List;

@Slf4j
public class ArenaCommandTest {

    @Test
    public void testCheckStatus() {
        List<String> commands = new ArrayList<>();
        commands.add("arena");
        commands.add("get");
        commands.add("tf-dist-git");

        log.info("arena get task: {}", StringUtils.join(commands, " "));

        ProcessBuilder pb = new ProcessBuilder();
        pb.command(commands);

        try {
            Process process = pb.start();
            try (BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()))) {
                StringBuilder sb = new StringBuilder();
                String line;
                while ((line = reader.readLine()) != null) {
                    if (line.startsWith("STATUS:")) {
                        String status = line.substring(line.indexOf(":") + 1).trim();
                        log.info(status);
                    }
                }
            }
        } catch (Exception e) {
            log.error("arena get task failed", e);
        }
    }

    @Test
    public void testString() {
        String str = "oss://cloudnativeai/arena/dataset/fashion-mnist/";
        int len = "oss//cloudnativeai".length();
        System.out.println(str.substring(len));
    }

    @Test
    public void testSubstr() {
        String path = "/data/arena/models";
        path = path.substring("/data".length());
        System.out.println(path);
    }

}
