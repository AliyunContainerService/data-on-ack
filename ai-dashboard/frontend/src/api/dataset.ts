import { get, post, put } from './client'

export const fetchDatasets = (namespace?: string) =>
  get<{ items: any[]; total: number }>('/dataset/list', { params: { namespace } })

export const createDataset = (data: any) => post<any>('/dataset/create', data)
export const deleteDataset = (name: string, namespace: string) =>
  put<any>('/dataset/delete', { params: { name, namespace } })
export const updateDataset = (data: any) => post<any>('/dataset/update', data)
