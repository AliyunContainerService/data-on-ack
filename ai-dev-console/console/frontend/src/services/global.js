import request from '@/utils/request';

export async function loginByRam() {
  return request('/api/v1/login-by-ram');
}

export async function queryCurrentUser() {
  return request('/api/v1/current-user');
}

export async function queryUserByToken(token) {
  return request('/api/v1/login/oauth2/token', {
    params: { token }
  });
}

export async function queryConfig() {
  return request('/api/v1/dlc/common-config');
}
export async function queryNamespaces() {
  return request('/api/v1/dlc/namespaces');
}
