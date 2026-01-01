import axios, { type AxiosInstance, type AxiosRequestConfig, type AxiosResponse, type AxiosError } from 'axios'

// Token 管理
const TOKEN_KEY = 'gotunnel_token'

export const getToken = (): string | null => localStorage.getItem(TOKEN_KEY)
export const setToken = (token: string): void => localStorage.setItem(TOKEN_KEY, token)
export const removeToken = (): void => localStorage.removeItem(TOKEN_KEY)

// 统一 API 响应结构
export interface ApiResponse<T = any> {
  code: number
  data?: T
  message?: string
}

// 业务错误码
export const ErrorCodes = {
  Success: 0,
  BadRequest: 400,
  Unauthorized: 401,
  Forbidden: 403,
  NotFound: 404,
  Conflict: 409,
  InternalError: 500,
  BadGateway: 502,
  ClientNotOnline: 1001,
  PluginNotFound: 1002,
  InvalidClientID: 1003,
  PluginDisabled: 1004,
  ConfigSyncFailed: 1005,
}

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

// 响应拦截器 - 处理统一响应格式
instance.interceptors.response.use(
  (response: AxiosResponse<ApiResponse>) => {
    const apiResponse = response.data

    // 检查业务错误码
    if (apiResponse.code !== undefined && apiResponse.code !== ErrorCodes.Success) {
      // 处理认证错误
      if (apiResponse.code === ErrorCodes.Unauthorized && !isRedirecting) {
        isRedirecting = true
        removeToken()
        setTimeout(() => {
          window.location.replace('/login')
          isRedirecting = false
        }, 0)
      }

      // 返回包含业务错误信息的 rejected promise
      return Promise.reject({
        code: apiResponse.code,
        message: apiResponse.message || 'Unknown error',
        response: response
      })
    }

    // 成功时返回 data 字段
    return {
      ...response,
      data: apiResponse.data !== undefined ? apiResponse.data : apiResponse
    } as AxiosResponse
  },
  (error: AxiosError<ApiResponse>) => {
    // 处理 HTTP 错误
    if (error.response?.status === 401 && !isRedirecting) {
      isRedirecting = true
      removeToken()
      setTimeout(() => {
        window.location.replace('/login')
        isRedirecting = false
      }, 0)
    }

    // 尝试从响应中提取业务错误信息
    const apiResponse = error.response?.data
    if (apiResponse?.message) {
      return Promise.reject({
        code: apiResponse.code || error.response?.status,
        message: apiResponse.message,
        response: error.response
      })
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
