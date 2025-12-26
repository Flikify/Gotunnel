import axios from 'axios'
import type { ClientConfig, ClientStatus, ClientDetail, ServerStatus, PluginInfo } from '../types'

const api = axios.create({
  baseURL: '/api',
  timeout: 10000,
})

// Token 管理
const TOKEN_KEY = 'gotunnel_token'

export const getToken = () => localStorage.getItem(TOKEN_KEY)
export const setToken = (token: string) => localStorage.setItem(TOKEN_KEY, token)
export const removeToken = () => localStorage.removeItem(TOKEN_KEY)

// 请求拦截器：添加 token
api.interceptors.request.use((config) => {
  const token = getToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器：处理 401
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      removeToken()
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

// 认证 API
export const login = (username: string, password: string) =>
  api.post<{ token: string }>('/auth/login', { username, password })
export const checkAuth = () => api.get('/auth/check')

export const getServerStatus = () => api.get<ServerStatus>('/status')
export const getClients = () => api.get<ClientStatus[]>('/clients')
export const getClient = (id: string) => api.get<ClientDetail>(`/client/${id}`)
export const addClient = (client: ClientConfig) => api.post('/clients', client)
export const updateClient = (id: string, client: ClientConfig) => api.put(`/client/${id}`, client)
export const deleteClient = (id: string) => api.delete(`/client/${id}`)
export const reloadConfig = () => api.post('/config/reload')

// 客户端控制
export const pushConfigToClient = (id: string) => api.post(`/client/${id}/push`)
export const disconnectClient = (id: string) => api.post(`/client/${id}/disconnect`)
export const installPluginsToClient = (id: string, plugins: string[]) =>
  api.post(`/client/${id}/install-plugins`, { plugins })

// 插件管理
export const getPlugins = () => api.get<PluginInfo[]>('/plugins')
export const enablePlugin = (name: string) => api.post(`/plugin/${name}/enable`)
export const disablePlugin = (name: string) => api.post(`/plugin/${name}/disable`)

export default api
