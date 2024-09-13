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

import com.aliyun.kubeai.model.auth.AKInfo;
import com.aliyun.kubeai.model.auth.RoleAuth;
import com.google.common.base.Strings;
import lombok.extern.slf4j.Slf4j;
import org.apache.commons.codec.binary.Base64;
import org.apache.commons.io.FileUtils;
import org.springframework.stereotype.Component;

import javax.annotation.Resource;
import javax.crypto.Cipher;
import javax.crypto.spec.IvParameterSpec;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.Charset;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.Arrays;

@Component
@Slf4j
public class AuthUtil {
    @Resource
    MetadataClient metadataClient;

    private static final String CONFIG_PATH = "/var/addon/token-config";

    public AKInfo getAKInfo() throws Exception {
        AKInfo akInfo = null;
        Path path = Paths.get(CONFIG_PATH);
        if (Files.exists(path)) {
            log.info("get ak from addon");
            String encodeTokenConfig = new String(Files.readAllBytes(path));
            if (Strings.isNullOrEmpty(encodeTokenConfig)) {
                log.error("token-config is empty");
                return null;
            }

            akInfo = new AKInfo(encodeTokenConfig);
            String keyring = akInfo.getKeyring();

            String akId = decrypt(akInfo.getAccessKeyId(), keyring);
            String akSecret = decrypt(akInfo.getAccessKeySecret(), keyring);
            String token = decrypt(akInfo.getSecurityToken(), keyring);

            akInfo.setAccessKeyId(akId);
            akInfo.setAccessKeySecret(akSecret);
            akInfo.setSecurityToken(token);
        } else {
            log.info("get ak from metadata");
            String roleName = metadataClient.getRoleName();
            if (Strings.isNullOrEmpty(roleName)) {
                log.error("cannot find role name");
                return null;
            }

            RoleAuth roleAuth = metadataClient.getRoleAuth(roleName);
            if (roleAuth != null && roleAuth.getCode().equals("Success")) {
                akInfo = new AKInfo();
                akInfo.setAccessKeyId(roleAuth.getAccessKeyId());
                akInfo.setAccessKeySecret(roleAuth.getAccessKeySecret());
                akInfo.setSecurityToken(roleAuth.getSecurityToken());
                akInfo.setExpiration(roleAuth.getExpiration());
            }
        }

        return akInfo;
    }

    private static String decrypt(String encrypted, String key) throws Exception {
        Cipher cipher = Cipher.getInstance("AES/CBC/PKCS5PADDING");
        SecretKeySpec secretKeySpec = new SecretKeySpec(key.getBytes(), "AES");

        int blockSize = cipher.getBlockSize();
        byte[] encryptedBytes = Base64.decodeBase64(encrypted);
        byte[] iv = Arrays.copyOfRange(encryptedBytes, 0, blockSize);
        IvParameterSpec ivSpec = new IvParameterSpec(iv);

        cipher.init(Cipher.DECRYPT_MODE, secretKeySpec, ivSpec);

        byte[] originBytes = Arrays.copyOfRange(encryptedBytes, blockSize, encryptedBytes.length);

        byte[] original = cipher.doFinal(originBytes);
        return new String(original, StandardCharsets.UTF_8);
    }
}
