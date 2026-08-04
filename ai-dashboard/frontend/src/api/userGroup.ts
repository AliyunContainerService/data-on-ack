import { get, post, put } from './client'

export interface UserGroup {
  metadata: { name: string; namespace: string }
  spec: {
    groupName: string
    quotaNames: string[]
    defaultRoles: string[]
    defaultClusterRoles: string[]
  }
}

export const fetchUserGroups = () => get<UserGroup[]>('/user_group/list')
export const createUserGroup = (data: any) => post<any>('/user_group/create', data)
export const updateUserGroup = (data: any) => put<any>('/user_group/update', data)
export const deleteUserGroup = (groupName: string) => put<any>('/user_group/delete', { params: { groupName } })
export const fetchGroupNamespaces = (groupName: string) =>
  get<string[]>('/user_group/get_group_namespaces', { params: { groupName } })
