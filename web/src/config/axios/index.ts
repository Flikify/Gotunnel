import axios, { type AxiosInstance, type AxiosRequestConfig, type AxiosResponse } from 'axios'

// Token 管理
const TOKEN_KEY = 'gotunnel_token'

export const getToken = (): string | null => localStorage.getItem(TOKEN_KEY)
export const setToken = (token: string): void => localStorage.setItem(TOKEN_KEY, token)
export const removeToken = (): void => localStorage.removeItem(TOKEN_KEY)

// 创建 axios 实例
const instance: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 10000,
})

// 防止重复跳转
let isRedirecting = false

// 请求拦截器
instance.interceptors.request.use(
  (config) => {
    const token = getToken()
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器
instance.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401 && !isRedirecting) {
      isRedirecting = true
      removeToken()
      setTimeout(() => {
        window.location.replace('/login')
        isRedirecting = false
      }, 0)
    }
    return Promise.reject(error)
  }
)

// 请求方法封装
export const get = <T = any>(
  url: string,
  config?: AxiosRequestConfig
): Promise<AxiosResponse<T>> => {
  return instance.get<T>(url, config)
}

export const post = <T = any>(
  url: string,
  data?: any,
  config?: AxiosRequestConfig
): Promise<AxiosResponse<T>> => {
  return instance.post<T>(url, data, config)
}

export const put = <T = any>(
  url: string,
  data?: any,
  config?: AxiosRequestConfig
): Promise<AxiosResponse<T>> => {
  return instance.put<T>(url, data, config)
}

export const del = <T = any>(
  url: string,
  config?: AxiosRequestConfig
): Promise<AxiosResponse<T>> => {
  return instance.delete<T>(url, config)
}

export const patch = <T = any>(
  url: string,
  data?: any,
  config?: AxiosRequestConfig
): Promise<AxiosResponse<T>> => {
  return instance.patch<T>(url, data, config)
}

export default instance
