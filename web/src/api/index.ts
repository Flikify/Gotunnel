import { get, post, put, del, getToken } from '../config/axios'
import type { ClientConfig, ClientStatus, ClientDetail, ServerStatus, PluginInfo, StorePluginInfo, PluginConfigResponse, JSPlugin, RuleSchemasMap, LogEntry, LogStreamOptions, ConfigField } from '../types'

// 重新导出 token 管理方法
export { getToken, setToken, removeToken } from '../config/axios'

// 认证 API
export const login = (username: string, password: string) =>
  post<{ token: string }>('/auth/login', { username, password })
export const checkAuth = () => get('/auth/check')

// 服务器状态
export const getServerStatus = () => get<ServerStatus>('/status')

// 客户端管理
export const getClients = () => get<ClientStatus[]>('/clients')
export const getClient = (id: string) => get<ClientDetail>(`/client/${id}`)
export const addClient = (client: ClientConfig) => post('/clients', client)
export const updateClient = (id: string, client: ClientConfig) => put(`/client/${id}`, client)
export const deleteClient = (id: string) => del(`/client/${id}`)
export const reloadConfig = () => post('/config/reload')

// 客户端控制
export const pushConfigToClient = (id: string) => post(`/client/${id}/push`)
export const disconnectClient = (id: string) => post(`/client/${id}/disconnect`)
export const restartClient = (id: string) => post(`/client/${id}/restart`)
export const installPluginsToClient = (id: string, plugins: string[]) =>
  post(`/client/${id}/install-plugins`, { plugins })

// 规则配置模式
export const getRuleSchemas = () => get<RuleSchemasMap>('/rule-schemas')

// 客户端插件控制（使用 pluginID）
export const startClientPlugin = (clientId: string, pluginId: string, ruleName: string) =>
  post(`/client/${clientId}/plugin/${pluginId}/start`, { rule_name: ruleName })
export const stopClientPlugin = (clientId: string, pluginId: string, ruleName: string) =>
  post(`/client/${clientId}/plugin/${pluginId}/stop`, { rule_name: ruleName })
export const restartClientPlugin = (clientId: string, pluginId: string, ruleName: string) =>
  post(`/client/${clientId}/plugin/${pluginId}/restart`, { rule_name: ruleName })
export const deleteClientPlugin = (clientId: string, pluginId: string) =>
  post(`/client/${clientId}/plugin/${pluginId}/delete`)
export const updateClientPluginConfigWithRestart = (clientId: string, pluginId: string, ruleName: string, config: Record<string, string>, restart: boolean) =>
  post(`/client/${clientId}/plugin/${pluginId}/config`, { rule_name: ruleName, config, restart })

// 插件管理
export const getPlugins = () => get<PluginInfo[]>('/plugins')
export const enablePlugin = (name: string) => post(`/plugin/${name}/enable`)
export const disablePlugin = (name: string) => post(`/plugin/${name}/disable`)

// 扩展商店
export const getStorePlugins = () => get<{ plugins: StorePluginInfo[] }>('/store/plugins')
export const installStorePlugin = (
  pluginName: string,
  downloadUrl: string,
  signatureUrl: string,
  clientId: string,
  remotePort?: number,
  version?: string,
  configSchema?: ConfigField[],
  authEnabled?: boolean,
  authUsername?: string,
  authPassword?: string
) =>
  post('/store/install', {
    plugin_name: pluginName,
    version: version || '',
    download_url: downloadUrl,
    signature_url: signatureUrl,
    client_id: clientId,
    remote_port: remotePort || 0,
    config_schema: configSchema || [],
    auth_enabled: authEnabled || false,
    auth_username: authUsername || '',
    auth_password: authPassword || ''
  })

// 客户端插件配置
export const getClientPluginConfig = (clientId: string, pluginName: string) =>
  get<PluginConfigResponse>(`/client-plugin/${clientId}/${pluginName}/config`)
export const updateClientPluginConfig = (clientId: string, pluginName: string, config: Record<string, string>) =>
  put(`/client-plugin/${clientId}/${pluginName}/config`, { config })

// JS 插件管理
export const getJSPlugins = () => get<JSPlugin[]>('/js-plugins')
export const createJSPlugin = (plugin: JSPlugin) => post('/js-plugins', plugin)
export const getJSPlugin = (name: string) => get<JSPlugin>(`/js-plugin/${name}`)
export const updateJSPlugin = (name: string, plugin: JSPlugin) => put(`/js-plugin/${name}`, plugin)
export const deleteJSPlugin = (name: string) => del(`/js-plugin/${name}`)
export const pushJSPluginToClient = (pluginName: string, clientId: string, remotePort?: number) =>
  post(`/js-plugin/${pluginName}/push/${clientId}`, { remote_port: remotePort || 0 })
export const updateJSPluginConfig = (name: string, config: Record<string, string>) =>
  put(`/js-plugin/${name}/config`, { config })
export const setJSPluginEnabled = (name: string, enabled: boolean) =>
  post(`/js-plugin/${name}/${enabled ? 'enable' : 'disable'}`)

// 插件 API 代理（通过 pluginID 调用插件自定义 API）
export const callPluginAPI = <T = any>(clientId: string, pluginId: string, method: string, route: string, body?: any) => {
  const path = `/client/${clientId}/plugin-api/${pluginId}${route.startsWith('/') ? route : '/' + route}`
  switch (method.toUpperCase()) {
    case 'GET':
      return get<T>(path)
    case 'POST':
      return post<T>(path, body)
    case 'PUT':
      return put<T>(path, body)
    case 'DELETE':
      return del<T>(path)
    default:
      return get<T>(path)
  }
}

// 更新管理
export interface UpdateInfo {
  available: boolean
  current: string
  latest: string
  release_note: string
  download_url: string
  asset_name: string
  asset_size: number
}

export interface VersionInfo {
  version: string
  git_commit: string
  build_time: string
  go_version: string
  os: string
  arch: string
}

export const getVersionInfo = () => get<VersionInfo>('/update/version')
export const checkServerUpdate = () => get<UpdateInfo>('/update/check/server')
export const checkClientUpdate = (os?: string, arch?: string) => {
  const params = new URLSearchParams()
  if (os) params.append('os', os)
  if (arch) params.append('arch', arch)
  const query = params.toString()
  return get<UpdateInfo>(`/update/check/client${query ? '?' + query : ''}`)
}
export const applyServerUpdate = (downloadUrl: string, restart: boolean = true) =>
  post('/update/apply/server', { download_url: downloadUrl, restart })
export const applyClientUpdate = (clientId: string, downloadUrl: string) =>
  post('/update/apply/client', { client_id: clientId, download_url: downloadUrl })

// 日志流
export const createLogStream = (
  clientId: string,
  options: LogStreamOptions = {},
  onLog: (entry: LogEntry) => void,
  onError?: (error: Event) => void
): EventSource => {
  const token = getToken()
  const params = new URLSearchParams()
  if (token) params.append('token', token)
  if (options.lines !== undefined) params.append('lines', String(options.lines))
  if (options.follow !== undefined) params.append('follow', String(options.follow))
  if (options.level) params.append('level', options.level)

  const url = `/api/client/${clientId}/logs?${params.toString()}`
  const eventSource = new EventSource(url)

  eventSource.addEventListener('log', (event) => {
    try {
      const entry = JSON.parse((event as MessageEvent).data) as LogEntry
      onLog(entry)
    } catch (e) {
      console.error('Failed to parse log entry', e)
    }
  })

  eventSource.addEventListener('heartbeat', () => {
    // Keep-alive, no action needed
  })

  if (onError) {
    eventSource.onerror = onError
  }

  return eventSource
}

// 流量统计
export interface TrafficStats {
  traffic_24h: { inbound: number; outbound: number }
  traffic_total: { inbound: number; outbound: number }
}

export interface TrafficRecord {
  timestamp: number
  inbound: number
  outbound: number
}

export const getTrafficStats = () => get<TrafficStats>('/traffic/stats')
export const getTrafficHourly = () => get<{ records: TrafficRecord[] }>('/traffic/hourly')

// 客户端系统状态
export interface SystemStats {
  cpu_usage: number
  memory_total: number
  memory_used: number
  memory_usage: number
  disk_total: number
  disk_used: number
  disk_usage: number
}

export const getClientSystemStats = (clientId: string) => get<SystemStats>(`/client/${clientId}/system-stats`)

// 服务器配置
export interface ServerConfigInfo {
  bind_addr: string
  bind_port: number
  token: string
  heartbeat_sec: number
  heartbeat_timeout: number
}

export interface WebConfigInfo {
  enabled: boolean
  bind_port: number
  username: string
  password: string
}

export interface PluginStoreConfigInfo {
  url: string
}

export interface ServerConfigResponse {
  server: ServerConfigInfo
  web: WebConfigInfo
  plugin_store: PluginStoreConfigInfo
}

export interface UpdateServerConfigRequest {
  server?: Partial<ServerConfigInfo>
  web?: Partial<WebConfigInfo>
  plugin_store?: Partial<PluginStoreConfigInfo>
}

export const getServerConfig = () => get<ServerConfigResponse>('/config')
export const updateServerConfig = (config: UpdateServerConfigRequest) => put('/config', config)
