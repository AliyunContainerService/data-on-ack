import request from '@/utils/request'

export function fetchTrainingJobList(query) {
  return request({
    url: '/job/training',
    method: 'get',
    params: query
  })
}

export function fetchServingJobList(query) {
  return request({
    url: '/job/serving',
    method: 'get',
    params: query
  })
}

export function fetchJobCost(query) {
  return request({
    url: '/job/cost',
    method: 'get',
    params: query
  })
}
