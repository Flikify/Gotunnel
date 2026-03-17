import { get, post, put, del, getToken } from '../config/axios'
import type { ClientConfig, ClientStatus, ClientDetail, ServerStatus, LogEntry, LogStreamOptions, InstallCommandResponse } from '../types'

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

// 客户端截图
export interface ScreenshotData {
  data: string      // Base64 JPEG
  width: number
  height: number
  timestamp: number
  error?: string
}

export const getClientScreenshot = (clientId: string, quality?: number) =>
  get<ScreenshotData>(`/client/${clientId}/screenshot${quality ? '?quality=' + quality : ''}`)

// Shell 执行
export interface ShellResult {
  output: string
  exit_code: number
  error?: string
}

export const executeClientShell = (clientId: string, command: string, timeout?: number) =>
  post<ShellResult>(`/client/${clientId}/shell`, { command, timeout: timeout || 30 })

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

// 安装命令生成
export const generateInstallCommand = (clientId: string) =>
  post<InstallCommandResponse>('/install/generate', { client_id: clientId })
