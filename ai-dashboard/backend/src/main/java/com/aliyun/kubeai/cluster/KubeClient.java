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

import com.aliyun.kubeai.exception.K8sCRDNotFoundException;
import com.google.common.base.Strings;
import io.fabric8.kubernetes.api.model.*;
import io.fabric8.kubernetes.api.model.apiextensions.v1.CustomResourceDefinition;
import io.fabric8.kubernetes.api.model.networking.v1.Ingress;
import io.fabric8.kubernetes.api.model.networking.v1.IngressList;
import io.fabric8.kubernetes.client.Config;
import io.fabric8.kubernetes.client.ConfigBuilder;
import io.fabric8.kubernetes.client.DefaultKubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.Resource;
import io.fabric8.kubernetes.client.dsl.base.CustomResourceDefinitionContext;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

import javax.annotation.PostConstruct;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

import static java.util.stream.Collectors.toList;

@Slf4j
@Component
public class KubeClient {

    private Config config;
    private KubernetesClient kubernetesClient;

    private static final String DEFAULT_NAMESPACE = "default";

    @PostConstruct
    public void init() {
        try {
            config = new ConfigBuilder().build();
            kubernetesClient = new DefaultKubernetesClient(config);
            log.info("init kubernetes client success");
        } catch (Exception e) {
            log.error("init kubernetes client failed", e);
            System.exit(-1);
        }
    }

    public KubernetesClient getClient() {
        return this.kubernetesClient;
    }

    public String getClusterId() {
        final String namespace = "kube-system";
        final String name = "ack-cluster-profile";
        Resource<ConfigMap> configMapResource = kubernetesClient.configMaps().inNamespace(namespace).withName(name);

        ConfigMap configMap = configMapResource.get();
        Map<String, String> data = configMap.getData();
        log.info("clusterId: " + data.get("clusterid"));
        return data.get("clusterid");
    }

    public List<String> listNamespace() {
        List<String> res = new ArrayList<>();
        try {
            NamespaceList namespaceList = kubernetesClient.namespaces().list();
            res = namespaceList.getItems().stream().map(x -> x.getMetadata().getName()).collect(toList());
        } catch (Exception e) {
            log.warn("list group failed:{}", e.toString());
            System.exit(-1);
        }
        return res;
    }

    public String getIngressHostByName(String name, String ns) {
        String host = getIngressV1HostByName(name, ns);
        if(Strings.isNullOrEmpty(host)) {
            host = getIngressV1beta1HostByName(name, ns);
        }
        return host;
    }

    public String getIngressV1HostByName(String name, String ns) {
        Ingress ingress;
        if (Strings.isNullOrEmpty(ns)) {
            IngressList ingressList = kubernetesClient.network().v1().ingresses().inAnyNamespace().list();
            if (ingressList == null) {
                return null;
            }
            List<Ingress> targetIngress = ingressList.getItems().stream().filter(x->x.getMetadata().getName().equals(name)).collect(Collectors.toList());
            if (null == targetIngress || targetIngress.isEmpty()) {
                return null;
            }
            log.info("found ingress size:{} by name:{}", targetIngress.size(), name);
            ingress = targetIngress.get(0);
        } else {
            ingress = kubernetesClient.network().v1().ingresses().inNamespace(ns).withName(name).get();
        }
        if (null == ingress || ingress.getSpec().getRules().isEmpty()) {
            log.warn("ingress not found name:{} ns:{}", name, ns);
            return null;
        }
        return ingress.getSpec().getRules().get(0).getHost();
    }

    public String getIngressV1beta1HostByName(String name, String ns) {
        io.fabric8.kubernetes.api.model.networking.v1beta1.Ingress ingress;
        if (Strings.isNullOrEmpty(ns)) {
            io.fabric8.kubernetes.api.model.networking.v1beta1.IngressList ingressList = kubernetesClient.network()
                    .ingresses().inAnyNamespace().list();
            if (ingressList == null) {
                return null;
            }
            List<io.fabric8.kubernetes.api.model.networking.v1beta1.Ingress> targetIngress = ingressList.getItems().
                    stream().filter(x->x.getMetadata().getName().equals(name)).collect(Collectors.toList());
            if (null == targetIngress || targetIngress.isEmpty()) {
                return null;
            }
            log.info("found ingress size:{} by name:{}", targetIngress.size(), name);
            ingress = targetIngress.get(0);
        } else {
            ingress = kubernetesClient.network().ingresses().inNamespace(ns).withName(name).get();
        }
        if (null == ingress || ingress.getSpec().getRules().isEmpty()) {
            log.warn("ingress not found name:{} ns:{}", name, ns);
            return null;
        }
        return ingress.getSpec().getRules().get(0).getHost();
    }

    public String getClusterIpServiceByName(String name, String ns) {
        Service service;
        if (Strings.isNullOrEmpty(ns)) {
            ServiceList serviceList = kubernetesClient.services().inAnyNamespace().list();
            if (serviceList == null || serviceList.getItems().isEmpty()) {
                return  null;
            }
            List<Service> targetServices = serviceList.getItems().stream().filter(x->x.getMetadata().getName().equals(name)).collect(Collectors.toList());
            if (targetServices == null || targetServices.isEmpty()) {
                return null;
            }
            log.info("found service size:{} by name:{}", targetServices.size(), name);
            service = targetServices.get(0);
        } else {
            service = kubernetesClient.services().inNamespace(ns).withName(name).get();
        }
        if (service == null) {
            log.warn("service not found name:{} ns:{}", name, ns);
            return null;
        }
        return service.getSpec().getClusterIP();
    }

    public CustomResourceDefinitionContext buildCrdContext(String crdName) throws K8sCRDNotFoundException {
        CustomResourceDefinition crd = this.kubernetesClient.apiextensions().v1().customResourceDefinitions()
                .withName(crdName)
                .get();
        if (crd == null) {
            throw new K8sCRDNotFoundException(crdName + " not found");
        }

        CustomResourceDefinitionContext ctx = new CustomResourceDefinitionContext().fromCrd(crd);
        return ctx;
    }

    public boolean deletePV(String pvName) {
        log.info("delete pv {}", pvName);
        return kubernetesClient.persistentVolumes().withName(pvName).delete();
    }

    public boolean deletePVC(String pvcName) {
        log.info("delete pvc {}", pvcName);
        return kubernetesClient.persistentVolumeClaims().inNamespace(DEFAULT_NAMESPACE).withName(pvcName).delete();
    }


    public ServiceAccount createServiceAccount(String namespace, String userName) throws Exception {
        ServiceAccount createdServiceAccount;

        ServiceAccount serviceAccount = new ServiceAccountBuilder()
                .withNewMetadata().withName(userName).endMetadata()
                .withAutomountServiceAccountToken(false)
                .build();
        createdServiceAccount = kubernetesClient.serviceAccounts().inNamespace(namespace).createOrReplace(serviceAccount);
        log.info("created service account:{}", createdServiceAccount);
        return createdServiceAccount;
    }

    public boolean createSecretForServiceAccount(String namespace, ServiceAccount serviceAccount) throws Exception {

        if (serviceAccount.getSecrets().size() != 0) {
            log.info("serivceaccount {} already has secret", serviceAccount.getMetadata().getName());
            return true;
        }
        Secret createdSecret;
        String serviceAccountName = serviceAccount.getMetadata().getName();

        // create  secret
        Secret secret = new SecretBuilder()
                .withNewMetadata().withName(serviceAccountName + "-token").addToAnnotations("kubernetes.io/service-account.name",serviceAccountName).endMetadata()
                .withNewType("kubernetes.io/service-account-token")
                .build();
        createdSecret = kubernetesClient.secrets().inNamespace(namespace).createOrReplace(secret);
        if (createdSecret == null) {
            log.error("fail to create secret for {}", serviceAccountName);
            return false;
        }

        // add secret reference to service account
        ObjectReference secretRef = new ObjectReference();
        secretRef.setName(createdSecret.getMetadata().getName());
        secretRef.setNamespace(namespace);
        secretRef.setApiVersion(createdSecret.getApiVersion());
        secretRef.setKind(createdSecret.getKind());
        List<ObjectReference> secretsList =  serviceAccount.getSecrets();
        secretsList.add(secretRef);
        serviceAccount.setSecrets(secretsList);
        ServiceAccount replacedSa = kubernetesClient.serviceAccounts().inNamespace(namespace).createOrReplace(serviceAccount);
        if (replacedSa == null) {
            log.error("fail add secret {} to serviceaccount {}", createdSecret.getMetadata().getName(), serviceAccountName);
            return false;
        }
        log.info("created secret {} for {}", createdSecret.getMetadata().getName(), serviceAccountName);
        return true;
    }


    public ServiceAccount createServiceAccountWithAutoMount(String namespace, String userName) throws Exception {
        ServiceAccount createdServiceAccount;

        ServiceAccount serviceAccount = new ServiceAccountBuilder()
                .withNewMetadata().withName(userName).endMetadata()
                .withAutomountServiceAccountToken(true)
                .build();
        createdServiceAccount = kubernetesClient.serviceAccounts().inNamespace(namespace).createOrReplace(serviceAccount);
        log.info("created service account:{}", createdServiceAccount);
        return createdServiceAccount;
    }
    public boolean deleteServiceAccount(String namespace, String name) {
        try {
            return kubernetesClient.serviceAccounts().inNamespace(namespace).withName(name).delete();
        } catch (Exception e) {
            log.error("delete service account error", e);
            return false;
        }
    }


    public boolean isPodExist(String namespace, String name) {
        try {
            Pod pod = kubernetesClient.pods().inNamespace(namespace).withName(name).get();
            if (pod != null) {
                return true;
            }
        } catch (Exception e) {
            log.error("get pod failed", e);
        }
        return false;
    }
}
