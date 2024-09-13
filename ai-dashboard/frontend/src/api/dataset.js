import request from '@/utils/request'

export function fetchDatasetList(query) {
  return request({
    url: '/dataset/list',
    method: 'get',
    params: query
  })
}

export function createDataset(data) {
  return request({
    url: '/dataset/create',
    method: 'post',
    data: data
  })
}

export function deleteDataset(data) {
  return request({
    url: '/dataset/delete',
    method: 'put',
    params: { name: data.name, namespace: data.namespace },
    data: {}
  })
}

export function updateDataset(data) {
  return request({
    url: '/dataset/update',
    method: 'post',
    data: data
  })
}
