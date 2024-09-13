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

import com.aliyun.kubeai.dao.EqTreeDao;
import com.aliyun.kubeai.dao.K8sUserGroupDao;
import com.aliyun.kubeai.model.k8s.eqtree.ElasticQuotaNodeWithPrefix;
import com.aliyun.kubeai.model.k8s.eqtree.ElasticQuotaTreeWithPrefix;
import com.aliyun.kubeai.service.K8sService;
import com.aliyun.kubeai.service.QuotaGroupService;
import com.google.common.base.Strings;
import io.fabric8.kubernetes.api.model.rbac.ClusterRole;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.context.event.ApplicationReadyEvent;
import org.springframework.context.event.EventListener;
import org.springframework.core.env.Environment;
import org.springframework.core.io.ClassPathResource;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;
import org.springframework.stereotype.Component;

import javax.sql.DataSource;
import java.io.IOException;
import java.io.InputStream;

@Slf4j
@Component
public class InitializeConfig {
    public final static String ElasticTreeName = "elasticquotatree";

    @Autowired
    private DataSource dataSource;

    @Autowired
    private KubeClient kubeClient;

    @Autowired
    private K8sUserGroupDao userGroupDao;

    @Autowired
    private EqTreeDao eqTreeDao;

    @Autowired
    private Environment env;

    @Autowired
    private QuotaGroupService groupService;

    @Autowired
    private K8sService k8sService;

    @EventListener(ApplicationReadyEvent.class)
    public void init() throws Exception{
        String host = env.getProperty("MYSQL_HOST");
        String dbName = env.getProperty("MYSQL_DB_NAME");
        log.info("init mysql, {}:{}", host, dbName);
        initDatabase();
        initDefaultEqTree();
        initDefaultUserGroup();
        initClusterRole();
        updatePipelineRBAC();
        updateResearcherRole();
    }

    public void initDefaultUserGroup() throws Exception{
        try {
            userGroupDao.createUserGroupFromFile("defaultUserGroup.yaml", false);
        } catch (Exception e) {
            log.error("init default user group failed", e);
        }
    }

    public void initDefaultEqTree() throws Exception{
        try {
            eqTreeDao.createEqTreeFromFile("defaultEqTree.yaml", false);
        } catch (Exception e) {
            log.error("init default eqtree failed", e);
        }
    }

    /**
     * create tables in database if not exists
     */
    public void initDatabase() {
        log.info("init database");
        ResourceDatabasePopulator resourceDatabasePopulator = new ResourceDatabasePopulator(
                false,
                false,
                "UTF-8",
                new ClassPathResource("kubeai.sql"));

        try {
            resourceDatabasePopulator.execute(dataSource);
            log.info("init database success");
        } catch (Exception e) {
            log.error("init database failed", e);
        }
    }

    public void initClusterRole() {
        log.info("init apply clusterRole");
        try {
            InputStream defaultAdminClusterRole = new ClassPathResource("defaultAdminClusterRole.yaml").getInputStream();
            InputStream defaultResearcherClusterRole = new ClassPathResource("defaultResearcherClusterRole.yaml").getInputStream();
            ClusterRole defaultAdminClusterRoleObject = this.kubeClient.getClient().rbac().clusterRoles().load(defaultAdminClusterRole).get();
            ClusterRole defaultResearcherClusterRoleObject = this.kubeClient.getClient().rbac().clusterRoles().load(defaultResearcherClusterRole).get();
            if(defaultAdminClusterRoleObject == null) {
                log.error("Default Admin ClusterRole InputFileStream is null.");
                return;
            }
            if(defaultResearcherClusterRoleObject == null) {
                log.error("Default Researcher ClusterRole InputFileStream is null.");
                return;
            }
            Object defaultAdminClusterRoleUpdate = this.kubeClient.getClient().rbac().clusterRoles().createOrReplace(defaultAdminClusterRoleObject);
            if (defaultAdminClusterRoleUpdate == null){
                log.error("DefaultAdminClusterRole Update Failed.");
            }
            Object defaultResearcherClusterRoleUpdate = this.kubeClient.getClient().rbac().clusterRoles().createOrReplace(defaultResearcherClusterRoleObject);
            if (defaultResearcherClusterRoleUpdate == null){
                log.error("DefaultResearcherClusterRole Update Failed.");
            }
        } catch (IOException e){
            log.error("init clusterRole failed", e);
        }
    }

    public void updatePipelineRBAC() throws Exception{
        log.info("begin create updatePipelineRBAC");
        // get elastic tree
        ElasticQuotaTreeWithPrefix tree = groupService.getElasticQuotaTree(ElasticTreeName, null);
        if (tree == null) {
            throw new Exception(String.format("elastic quota tree not found name:%s", ElasticTreeName));
        }
        // get root node
        ElasticQuotaNodeWithPrefix node = tree.getSpec().getRoot();

        // update pieline RBAC
        groupService.updatePipelineRBAC(node);
        log.info("success create updatePipelineRBAC");
    }


    public void updateResearcherRole() throws Exception {
        log.info("begin updateResearcherRole");
        // get elastic tree
        ElasticQuotaTreeWithPrefix tree = groupService.getElasticQuotaTree(ElasticTreeName, null);
        if (tree == null) {
            throw new Exception(String.format("elastic quota tree not found name:%s", ElasticTreeName));
        }
        // get root node
        ElasticQuotaNodeWithPrefix node = tree.getSpec().getRoot();

        // update ResearchRole
        groupService.updateResearchRole(node);

        // update cluster role
        k8sService.updateResearchClusterRole();
        log.info("success  updateResearcherRole and ClusterRole");
    }


}
