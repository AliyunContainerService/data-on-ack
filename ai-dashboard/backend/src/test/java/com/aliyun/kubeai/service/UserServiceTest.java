package com.aliyun.kubeai.service;

import com.aliyun.kubeai.model.k8s.user.User;
import com.aliyun.kubeai.model.k8s.user.K8sRoleBinding;
import com.aliyun.kubeai.model.k8s.user.K8sServiceAccount;
import com.aliyun.kubeai.model.k8s.user.Spec;
import com.aliyun.kubeai.vo.ApiRole;
import com.google.common.base.Strings;
import lombok.extern.slf4j.Slf4j;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ContextConfiguration;
import org.springframework.test.context.junit4.SpringJUnit4ClassRunner;
import org.springframework.test.context.support.AnnotationConfigContextLoader;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

import static org.junit.Assert.assertTrue;

@SpringBootTest
@RunWith(SpringJUnit4ClassRunner.class)
@ContextConfiguration(loader = AnnotationConfigContextLoader.class, classes = ServiceTestContext.class)
@Slf4j
public class UserServiceTest {
    @Autowired
    UserService userService;

    @Autowired
    K8sService k8sService;

    @Test
    public void testAdminCRUD() {
        String aliuid = "testaliuid12345678910";
        String uid = "testuid12345678910";

        User newAdmin = new User();
        Spec spec = new Spec();
        String userName = uid;
        K8sServiceAccount serviceAccount = new K8sServiceAccount();
        serviceAccount.setName(k8sService.genServiceAccountName(null, uid));
        serviceAccount.setNamespace(k8sService.DEFAULT_USER_NAMESPACE);
        List<K8sRoleBinding> clusterRoledindings = new ArrayList<>();
        K8sRoleBinding roleBinding = new K8sRoleBinding();
        roleBinding.setRoleName(k8sService.ADMIN_DEFAULT_CLUSTER_ROLE);
        clusterRoledindings.add(roleBinding);
        K8sServiceAccount k8sServiceAccount = new K8sServiceAccount();
        spec.setApiRoles(Arrays.asList(ApiRole.ADMIN.toString()));
        spec.setUserName(userName);
        spec.setPassword("123456");
        spec.setAliuid(aliuid);
        spec.setDeletable(true);
        k8sServiceAccount.setClusterRoleBindings(clusterRoledindings);
        spec.setK8sServiceAccount(k8sServiceAccount);
        newAdmin.setSpec(spec);
        log.info("create admin uid:{}", uid);
        User oldUser = userService.findUserByAliuid(aliuid);
        if (oldUser != null) {
            try {
                assertTrue(userService.deleteUser(oldUser));
            } catch (Exception e) {
                log.error("delete user error:", e);
                assertTrue(false);
            }
        }

        try {
            assertTrue(userService.createUser(newAdmin));
        } catch (Exception e) {
            log.error("create user error:", e);
            assertTrue(false);
        }

        User user = userService.findUserByAliuid(aliuid);
        assertTrue(user != null);
        String ramUserName = "root";
        // set user name
        String oldUserName = user.getSpec().getUserName();
        if (Strings.isNullOrEmpty(oldUserName) || !oldUserName.equals(ramUserName)) {
            user.getSpec().setUserName(ramUserName);
            user.getMetadata().setCreationTimestamp(null);
            try {
                assertTrue(userService.updateUser(user));
                log.info("upadate user name ok userName from:{} to:{}", userName, ramUserName);
            } catch (Exception e) {
                log.error("update user error:", e);
                assertTrue(false);
            }
        }
    }

}
