import axios, { type AxiosRequestConfig } from 'axios'

const client = axios.create({
  baseURL: '/',
  timeout: 30000,
  withCredentials: true,
})

client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401 || error.response?.status === 302) {
      window.location.href = '/login'
    }
    return Promise.reject(error)
  },
)

export async function get<T = any>(url: string, config?: AxiosRequestConfig): Promise<T> {
  const res = await client.get(url, config)
  const data = res.data
  if (data?.code === 10000) return data.data
  if (data?.code && [10101, 10102, 10103].includes(data.code)) {
    window.location.href = '/login'
  }
  throw data
}

export async function post<T = any>(url: string, body?: any, config?: AxiosRequestConfig): Promise<T> {
  const res = await client.post(url, body, config)
  const data = res.data
  if (data?.code === 10000) return data.data
  throw data
}

export async function put<T = any>(url: string, body?: any, config?: AxiosRequestConfig): Promise<T> {
  const res = await client.put(url, body, config)
  const data = res.data
  if (data?.code === 10000) return data.data
  throw data
}

export async function getBlob(url: string, config?: AxiosRequestConfig): Promise<Blob> {
  const res = await client.get(url, { ...config, responseType: 'blob' })
  return res.data as Blob
}

/** Extract a human-readable error message from any thrown value. */
export function getErrorMessage(err: unknown): string {
  if (!err) return 'Unknown error'
  if (typeof err === 'string') return err
  if (err instanceof Error) return err.message
  if (typeof err === 'object') {
    const obj = err as Record<string, any>
    return obj.message || obj.msg || obj.Message || JSON.stringify(obj)
  }
  return String(err)
}

export default client
