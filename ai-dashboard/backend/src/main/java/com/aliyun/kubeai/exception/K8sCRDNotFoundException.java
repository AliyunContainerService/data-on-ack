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
    
package com.aliyun.kubeai.exception;

public class K8sCRDNotFoundException extends RuntimeException {
    public K8sCRDNotFoundException() {
        super();
    }

    public K8sCRDNotFoundException(String message, Throwable cause) {
        super(message, cause);
    }

    public K8sCRDNotFoundException(String message) {
        super(message);
    }

    public K8sCRDNotFoundException(Throwable cause) {
        super(cause);
    }
}
