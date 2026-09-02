import axios, { AxiosResponse } from 'axios';

const http = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  withCredentials: true,
});

export interface ApiResponse<T = unknown> {
  code: number;
  message: string;
  data: T;
}

http.interceptors.response.use(
  (resp) => resp,
  (error) => {
    if (error.response?.status === 401) {
      window.location.href = '/login';
    }
    // Business failures arrive as HTTP 4xx with an envelope `{code, message}`
    // (e.g. "too many active runs", "session already has an active run").
    // Reject with the envelope message — mirroring the success-path envelope
    // handling below — instead of axios' opaque "Request failed with status code 4xx".
    const envelope = error.response?.data as ApiResponse | undefined;
    if (envelope && typeof envelope.message === 'string' && envelope.message) {
      return Promise.reject(new Error(envelope.message));
    }
    return Promise.reject(error);
  }
);

export async function get<T>(url: string, params?: Record<string, string>): Promise<T> {
  const resp: AxiosResponse<ApiResponse<T>> = await http.get(url, { params });
  if (resp.data.code !== 10000) throw new Error(resp.data.message);
  return resp.data.data;
}

export async function post<T>(url: string, data?: unknown): Promise<T> {
  const resp: AxiosResponse<ApiResponse<T>> = await http.post(url, data);
  if (resp.data.code !== 10000) throw new Error(resp.data.message);
  return resp.data.data;
}

export async function put<T>(url: string, data?: unknown): Promise<T> {
  const resp: AxiosResponse<ApiResponse<T>> = await http.put(url, data);
  if (resp.data.code !== 10000) throw new Error(resp.data.message);
  return resp.data.data;
}

export async function del<T>(url: string): Promise<T> {
  const resp: AxiosResponse<ApiResponse<T>> = await http.delete(url);
  if (resp.data.code !== 10000) throw new Error(resp.data.message);
  return resp.data.data;
}

export default http;
