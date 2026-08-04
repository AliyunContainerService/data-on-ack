import { get, post, put } from './client'

export interface QuotaNode {
  name: string
  min?: Record<string, string>
  max?: Record<string, string>
  children?: QuotaNode[]
  namespaces?: string[]
}

export interface QuotaTree {
  metadata: { name: string; namespace: string }
  spec: { root: QuotaNode }
}

export const fetchQuotaTrees = () => get<QuotaTree[]>('/group/list')
export const createQuotaTree = (data: any) => post<any>('/group/create', data)
export const updateQuotaTree = (params: Record<string, string>, data: any) =>
  put<any>('/group/update', data, { params })
