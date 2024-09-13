# -- coding: utf-8 --
import json
import logging
import yaml
from kubernetes import client, config

logging.basicConfig(filename='copy_kubeai_io_crs.log', filemode='w', format='%(name)s - %(levelname)s - %(message)s', level=logging.INFO)

# Configs can be set in Configuration class directly or using helper utility
config.load_kube_config('kube.config')

class Object(object):
    def toJSON(self):
        return json.dumps(self, default=lambda o: o.__dict__,
                          sort_keys=True, indent=4)
    pass

def getdotattr(dict, dotattr):
    attrs = dotattr.split(".")
    if len(attrs) == 1:
        return dict.get(attrs[0])
    return getdotattr(dict.get(attrs[0]), ".".join(attrs[1:]))

def setdotattr(object, dotattr, value):
    attrs = dotattr.split(".")
    if len(attrs) == 1:
        setattr(object, dotattr, value)
        return
    if not hasattr(object, attrs[0]):
        setattr(object, attrs[0], Object())
    setdotattr(getattr(object, attrs[0]), ".".join(attrs[1:]), value)
    return

def kubeAiIoUserToAlibabaCloudUser(kubeAiIoUser):
    attrs_to_reserve = ["apiVersion",
                        "group",
                        "crdName",
                        "kind",
                        "metadata.name",
                        "metadata.namespace",
                        "spec.aliuid",
                        "spec.apiRoles",
                        "spec.deletable",
                        "spec.userId",
                        "spec.userName",
                        "spec.k8sServiceAccount.name",
                        "spec.k8sServiceAccount.namespace",
                        "spec.k8sServiceAccount.clusterRoleBindings",
                        "spec.k8sServiceAccount.roleBindings"
                        ]
    attr_to_replace = {
        "apiVersion": "data.kubeai.alibabacloud.com/v1",
        "crdName": "users.data.kubeai.alibabacloud.com",
        "group": "data.kubeai.alibabacloud.com"
    }
    attr_to_add = {
        "spec.groups": [],
    }
    newUser = Object()
    for attr in attrs_to_reserve:
        value = getdotattr(kubeAiIoUser, attr) if isinstance(kubeAiIoUser, dict) else getattr(kubeAiIoUser, attr)
        if attr in attr_to_replace:
            value = attr_to_replace[attr]
        setdotattr(newUser, attr, value)
    for attr, value in attr_to_add.items():
        setdotattr(newUser, attr, value)

    return newUser

if __name__ == "__main__":
    configuration = client.Configuration.get_default_copy()
    logging.info("client configuration:%s", configuration.host)
    with client.ApiClient(configuration) as api_client:
        api_instance = client.CustomObjectsApi(api_client)
        alibabacloud_users = api_instance.list_namespaced_custom_object(group="data.kubeai.alibabacloud.com",
                                                                        version="v1",
                                                                        namespace="kube-ai",
                                                                        plural="users")
        alibabacloud_user_meta_names = [getdotattr(user, "metadata.name") for user in alibabacloud_users["items"]]

        with open("users.data.kubeai.io.checkpoint", "w") as of:
            try:
                kubeai_domain_users = api_instance.list_namespaced_custom_object(group="data.kubeai.io",
                                                                                 version="v1",
                                                                                 namespace="kube-ai",
                                                                                 plural="users")
                print("total user to copy:%d"%(len(kubeai_domain_users.get("items"))))
                for user in kubeai_domain_users.get("items"):
                    user_meta_name = getdotattr(user, "metadata.name")
                    print("copy user:%s" % user_meta_name)
                    logging.info("processing user:%s", user)
                    if user_meta_name in alibabacloud_user_meta_names:
                        print("skip user meta name:%s" % user_meta_name)
                        logging.info("skip user meta name:%s" % user_meta_name)
                        continue
                    alibabaCloudUser = kubeAiIoUserToAlibabaCloudUser(user)
                    yamlUser = yaml.safe_load(alibabaCloudUser.toJSON())
                    yamlUserStr = yaml.safe_dump(yamlUser)
                    of.writelines([yamlUserStr, "---", "\n"])
                    res = api_instance.create_namespaced_custom_object(group="data.kubeai.alibabacloud.com",
                                                                 version="v1",
                                                                 namespace="kube-ai",
                                                                 plural="users",
                                                                 body=yamlUser,
                                                                 async_req=False)
                    logging.info("create new user:%s", alibabaCloudUser.toJSON())
            except Exception as e:
                logging.exception("copy user exception", e)
        logging.info("copy user done")
