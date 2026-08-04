import { get, post, put, getBlob } from './client'

export interface RamUser {
  userId: string
  userName: string
  displayName: string
}

export interface UserSpec {
  userName: string
  userId: string
  aliuid: string
  apiRoles: string[]
  groups: string[]
  k8sServiceAccount?: {
    name: string
    namespace: string
    roleBindings: { roleName: string; namespace: string }[]
    clusterRoleBindings: { roleName: string; namespace: string }[]
  }
}

export interface K8sUser {
  metadata: { name: string; namespace: string; creationTimestamp: string }
  spec: UserSpec
}

export const fetchRamUsers = () => get<RamUser[]>('/user/list/ramUsers')
export const getUserInfo = () => get<any>('/user/info')
export const getUserByAliuid = (aliuid: string) => get<any>('/user/get', { params: { aliuid } })

export const fetchResearchers = (userName?: string) =>
  get<{ items: K8sUser[]; total: number }>('/researcher/list', { params: { userName } })

export const createResearcher = (data: any) => post<any>('/researcher/create', data)
export const updateResearcher = (data: any) => put<any>('/researcher/update', data)
export const deleteResearcher = (userId: string) => put<any>('/researcher/delete', { params: { userId } })

export const getBearerToken = (userId: string) => get<string>('/researcher/getBearerToken', { params: { userId } })
export const downloadKubeConfig = (userId: string, namespace?: string) =>
  getBlob('/researcher/download/kubeconfig', { params: { userId, namespace } })
