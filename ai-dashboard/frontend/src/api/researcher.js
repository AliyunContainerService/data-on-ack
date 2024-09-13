import request from '@/utils/request'
import { isEmpty, uniqObjectList } from '@/utils'
import axios from 'axios'

const DEFAULT_ADMIN_CLUSTER_ROLE = 'kubeai-admin-clusterrole'
const DEFAULT_RESEARCHER_ROLE = 'kubeai-researcher-role'
const DEFAULT_RESEARCHER_CLUSTER_ROLE = 'kubeai-researcher-clusterrole'
const USER_TYPE_ADMIN = 'admin'

function toK8sUser(user) {
  function toK8sRoleBingding(roleName, namespace) {
    return {
      roleName: roleName,
      namespace: namespace
    }
  }
  user = setRolesByUserType(user)
  var clusterRoles = []
  if (!isEmpty(user.clusterRoles)) {
    clusterRoles = user.clusterRoles.map(x => toK8sRoleBingding(x, null))
    clusterRoles = uniqObjectList(clusterRoles, ['roleName'])
  }
  var roles = []
  if (!isEmpty(user.roles) && !isEmpty(user.roleNamespaces)) {
    for (var i = 0; i < user.roles.length; i++) {
      var roleName = user.roles[i]
      for (var j = 0; j < user.roleNamespaces.length; j++) {
        var namespace = user.roleNamespaces[j]
        roles.push({ namespace: namespace, roleName: roleName })
      }
    }
    roles = uniqObjectList(roles, ['namespace', 'roleName'])
  }
  var k8sUser = {
    metadata: {
      name: user.uid
    },
    spec: {
      userName: user.userName,
      apiRoles: user.apiRoles.constructor === Array ? user.apiRoles : [user.apiRoles],
      groups: user.groups || [],
      uid: user.uid,
      aliuid: user.aliuid,
      k8sServiceAccount: {
        roleBindings: roles,
        clusterRoleBindings: clusterRoles
      }
    }
  }
  return k8sUser
}

export function setRolesByUserType(researcher) {
  var clusterRoles = []
  var roles = []
  roles.push(DEFAULT_RESEARCHER_ROLE)
  clusterRoles.push(DEFAULT_RESEARCHER_CLUSTER_ROLE)
  if (researcher.apiRoles.indexOf(USER_TYPE_ADMIN) >= 0) {
    clusterRoles.push(DEFAULT_ADMIN_CLUSTER_ROLE)
    roles.push(DEFAULT_RESEARCHER_ROLE)
  }
  researcher.roles = roles
  researcher.clusterRoles = clusterRoles
  return researcher
}

export function deserializeK8sUser(k8sUser) {
  const spec = k8sUser.spec
  const metadata = k8sUser.metadata
  var clusterRoles
  if (!isEmpty(spec.k8sServiceAccount) && !isEmpty(spec.k8sServiceAccount.clusterRoleBindings)) {
    clusterRoles = spec.k8sServiceAccount.clusterRoleBindings.map(x => x.roleName)
  }
  function deserializeK8sRoleBinding(k8sRoleBindings) {
    var res = []
    if (isEmpty(k8sRoleBindings)) {
      return res
    }
    res = new Map()
    for (var i = 0; i < k8sRoleBindings.length; i++) {
      var namespace = k8sRoleBindings[i].namespace
      if (!res.has(namespace)) {
        res.set(namespace, [])
      }
      res.get(namespace).push(k8sRoleBindings[i].roleName)
    }
    var ret = []
    res.forEach((v, k) => (ret.push({ namespace: k, roleNames: v })))
    return ret
  }
  var roles
  if (!isEmpty(spec.k8sServiceAccount) && !isEmpty(spec.k8sServiceAccount.roleBindings)) {
    roles = deserializeK8sRoleBinding(spec.k8sServiceAccount.roleBindings)
  }
  var user = {
    userName: spec.userName,
    uid: metadata.name,
    aliuid: spec.aliuid,
    apiRoles: spec.apiRoles,
    clusterRoles: clusterRoles,
    groups: spec.groups || [],
    roles: roles,
    createTime: metadata.creationTimestamp
  }
  return user
}
export function fetchRamUserList() {
  return request({
    url: '/user/list/ramUsers',
    method: 'get'
  })
}

export function getBearerTokenByUser(reseracher) {
  var params = {
    userId: reseracher.uid
  }
  return axios({
    url: '/researcher/getBearerToken',
    method: 'GET',
    params: params
  })
}

export function fetchResearcherList(query) {
  return request({
    url: '/researcher/list',
    method: 'get',
    params: query
  })
}

export function downloadK8sConfig(user) {
  var params = {
    userId: user.uid,
    namespace: user.namespace
  }

  return axios({
    url: '/researcher/download/kubeconfig',
    method: 'GET',
    responseType: 'blob',
    params: params
  })
}

export function deleteResearcher(user) {
  var data = toK8sUser(user)
  return request({
    url: '/researcher/delete',
    method: 'put',
    data: data
  })
}

export function updateResearcher(user) {
  var data = toK8sUser(user)
  return request({
    url: '/researcher/update',
    method: 'put',
    data: data
  })
}

export function createResearcher(user) {
  var data = toK8sUser(user)
  return request({
    url: '/researcher/create',
    method: 'post',
    data: data
  })
}
