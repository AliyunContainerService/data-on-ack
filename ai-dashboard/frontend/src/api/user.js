import request from '@/utils/request'

export function logoutAliyun() {
  return request({
    url: '/logout',
    metho: 'post'
  })
}

export function loginAliyun(data) {
  return request({
    url: 'login/aliyun',
    method: 'get',
    data
  })
}

export function getUserInfo() {
  return request({
    url: '/user/info',
    method: 'get'
  })
}

export function logout(token) {
  const data = {
    'token': token
  }
  return request({
    url: '/user/logout',
    method: 'post',
    data
  })
}
