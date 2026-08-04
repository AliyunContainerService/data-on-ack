import { get } from './client'

export const fetchPvcs = (namespace?: string) =>
  get<{ items: any[]; total: number }>('/k8s/pvc/list', { params: { namespace } })

export const fetchSecrets = (namespace?: string) =>
  get<{ items: any[]; total: number }>('/k8s/secret/list', { params: { namespace } })

export const fetchNamespaces = () =>
  get<{ items: any[]; total: number }>('/k8s/namespace/list')
