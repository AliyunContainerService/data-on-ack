import Cookies from 'js-cookie'

const AiDashboardTokenKey = 'ai_dashboard_token'
const AiDashboardUserRolesKey = 'ai_dashboard_user_roles'
const AiDashboardClusterInfoKey = 'ai_dashboard_cluster_info'

export function getToken() {
  var token = Cookies.get(AiDashboardTokenKey)
  if (token === 'undefined') {
    return undefined
  }
  return token
}

export function getTokenByKey(key) {
  return Cookies.get(key)
}

export function deleteCookieKey(key) {
  Cookies.remove(key)
}

export function setToken(token) {
  return Cookies.set(AiDashboardTokenKey, token)
}

export function setRoles(roles) {
  return Cookies.set(AiDashboardUserRolesKey, roles)
}

export function getRoles(roles) {
  return Cookies.get(AiDashboardUserRolesKey)
}

export function setClusterInfo(clusterInfo) {
  return Cookies.set(AiDashboardClusterInfoKey, clusterInfo)
}

export function getClusterInfo() {
  var clusterInfo = Cookies.get(AiDashboardClusterInfoKey)
  if (clusterInfo === 'undefined') {
    return undefined
  }
  return JSON.parse(clusterInfo)
}

export function setTokenByKey(key, token) {
  return Cookies.set(key, token)
}

export function removeToken() {
  return Cookies.remove(AiDashboardTokenKey)
}
