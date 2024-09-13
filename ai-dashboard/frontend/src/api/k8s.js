import request from '@/utils/request'

export function fetchPvcList(query) {
  return request({
    url: '/k8s/pvc/list',
    method: 'get',
    params: query
  })
}

export function fetchSecretList(query) {
  return request({
    url: '/k8s/secret/list',
    method: 'get',
    params: query
  })
}

export function fetchNamespaceList(query) {
  return request({
    url: '/k8s/namespace/list',
    method: 'get',
    params: query
  })
}
