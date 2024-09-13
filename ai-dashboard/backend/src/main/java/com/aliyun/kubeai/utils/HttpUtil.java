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
    
package com.aliyun.kubeai.utils;

import com.google.common.base.Strings;
import lombok.extern.slf4j.Slf4j;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;

import java.io.IOException;
import java.net.InetAddress;
import java.net.UnknownHostException;

@Slf4j
public class HttpUtil {
    public static Boolean isDomainAvailable(String domainName)  {
        if (Strings.isNullOrEmpty(domainName)) {
            return false;
        }
        InetAddress domainInetAddress = null;

        try {
            domainInetAddress = InetAddress.getByName(domainName);
            log.info("domain:{} address:{}", domainName, domainInetAddress);
            return true;
        } catch (UnknownHostException uhe) {
            log.error("got domain exception", uhe);
            return false;
        }
    }

    public static String get(String url) throws IOException {
        OkHttpClient client = new OkHttpClient();

        Request request = new Request.Builder()
                .url(url)
                .build();

        Response response = null;
        try {
            response = client.newCall(request).execute();
            if (response.code() == 200) {
                return response.body().string();
            }
        } catch (IOException e) {
            throw e;
        } finally {
            if (response != null) {
                response.body().close();
            }
        }
        return null;
    }

}
