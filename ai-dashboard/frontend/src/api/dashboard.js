import request from '@/utils/request'

export function fetchDashboardUrl(params) {
  return request({
    url: '/dashboard/url',
    method: 'get',
    params
  })
}
