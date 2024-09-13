import request from '@/utils/request'
import { isEmpty } from '@/utils'

export function fromK8sUserGroup(k8sUserGroup, users) {
  // const userNames = isEmpty(groupUsersMap) ? [] : groupUsersMap[groupName]
  const spec = k8sUserGroup.spec
  if (isEmpty(spec)) {
    return {
      name: '',
      userNames: [],
      quotaNames: ''
    }
  }
  const groupName = spec.groupName
  return {
    metaName: k8sUserGroup.metadata.name,
    metaNamepace: k8sUserGroup.metadata.namespace,
    name: groupName,
    userNames: users,
    quotaNames: isEmpty(spec.quotaNames) ? '' : spec.quotaNames[0]
  }
}

export function toK8sUserGroup(group) {
  const metaName = isEmpty(group.metaName) ? group.name : group.metaName
  const metaNamespace = isEmpty(group.metaNamespace) ? 'kube-ai' : group.metaNamespace
  const spec = {
    kind: 'UserGroup',
    apiVersion: 'data.kubeai.alibabacloud.com/v1',
    metadata: {
      name: metaName,
      namespace: metaNamespace
    },
    spec: {
      groupName: group.name
    }
  }

  if (group.quotaNames) {
    spec.spec.quotaNames = group.quotaNames.constructor === Array ? group.quotaNames : [group.quotaNames]
  }
  return spec
}

export function fetchUserGroup(query) {
  return request({
    url: '/user_group/list',
    method: 'get',
    params: query
  })
}

export function createUserGroup(group, userNames) {
  var data = toK8sUserGroup(group)
  return request({
    url: '/user_group/create',
    method: 'post',
    data: {
      userGroup: data,
      userNames
    }
  })
}

export function updateUserGroup(group, userList) {
  var data = {
    userGroup: toK8sUserGroup(group),
    users: userList
  }
  return request({
    url: '/user_group/update',
    method: 'put',
    data
  })
}

export function deleteUserGroup(group) {
  var data = toK8sUserGroup(group)
  return request({
    url: '/user_group/delete',
    method: 'put',
    data
  })
}

export function fetchUserGroupNamespaces(query) {
  return request({
    url: '/user_group/get_group_namespaces',
    method: 'get',
    params: query
  })
}

// export function updateUserForGroup(data) {
//  return request({
//    url: '/user_group/update_users',
//    method: 'PUT',
//    headers: { 'content-type': 'application/json' },
//    data: data
//  })
// }
//
