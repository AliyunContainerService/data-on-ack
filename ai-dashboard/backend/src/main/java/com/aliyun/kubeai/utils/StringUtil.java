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

import com.alibaba.fastjson.JSON;
import com.google.common.base.Strings;

import java.util.Arrays;
import java.util.List;

public class StringUtil {
    static public List<String> deserializeNodeName(String nodeNameWithPrefix) {
        List<String> listName = Arrays.asList(nodeNameWithPrefix.split("\\."));
        int prefixEndIndex = listName.size() - 2;
        String prefix = null;
        if (prefixEndIndex >= 0) {
            prefix = String.join(".", listName.subList(0, prefixEndIndex + 1));
        }
        return Arrays.asList(prefix, listName.get(listName.size() - 1));
    }

    static public String toJsonStringIfNotNull(Object object) {
        if (object == null)  {
            return "";
        }
        return JSON.toJSONString(object);
    }

    static public String serializeNodeName(String prefix, String nodeNameWithoutPrefix) {
        if (Strings.isNullOrEmpty(prefix)) {
            return nodeNameWithoutPrefix;
        }
        return prefix + "." + nodeNameWithoutPrefix;
    }

    static public String genCrdMetaKeys(String name, String namespace) {
        return String.format("%s.%s", name, namespace);
    }
}
