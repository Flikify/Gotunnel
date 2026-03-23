import { del, get, getToken, post, put } from '../config/axios'
import type { OperationRequestBody, OperationResult } from './generated'

export { getToken, removeToken, setToken } from '../config/axios'

type WithRequired<T, K extends keyof T> = T & { [P in K]-?: NonNullable<T[P]> }

export type ProxyRule = NonNullable<OperationRequestBody<'POST /api/clients'>['rules']>[number]
export type ClientConfig = OperationRequestBody<'POST /api/clients'>
export type UpdateClientPayload = OperationRequestBody<'PUT /api/client/{id}'>

type RawClientStatus = NonNullable<OperationResult<'GET /api/clients'>>[number]
export type ClientStatus = WithRequired<RawClientStatus, 'id' | 'online' | 'rule_count'>

type RawClientDetail = OperationResult<'GET /api/client/{id}'>
export type ClientDetail = WithRequired<RawClientDetail, 'id' | 'online' | 'rules'>

type RawServerStatus = OperationResult<'GET /api/status'>
type RawServerBindInfo = NonNullable<RawServerStatus['server']>
export type ServerStatus = WithRequired<Omit<RawServerStatus, 'server'>, 'client_count'> & {
  server: WithRequired<RawServerBindInfo, 'bind_addr' | 'bind_port'>
}

export type InstallCommandResponse = WithRequired<OperationResult<'POST /api/install/generate'>, 'expires_at' | 'token' | 'tunnel_port'>
export type UpdateInfo = WithRequired<OperationResult<'GET /api/update/check/server'>, 'available' | 'current'>
export type VersionInfo = WithRequired<OperationResult<'GET /api/update/version'>, 'arch' | 'go_version' | 'os' | 'version'>

type RawTrafficStats = OperationResult<'GET /api/traffic/stats'>
type RawTrafficTotals = NonNullable<RawTrafficStats['traffic_24h']>
type TrafficTotals = WithRequired<RawTrafficTotals, 'inbound' | 'outbound'>
export type TrafficStats = Omit<RawTrafficStats, 'traffic_24h' | 'traffic_total'> & {
  traffic_24h: TrafficTotals
  traffic_total: TrafficTotals
}

type RawHourlyTraffic = OperationResult<'GET /api/traffic/hourly'>
export type TrafficRecord = WithRequired<NonNullable<RawHourlyTraffic['records']>[number], 'inbound' | 'outbound' | 'timestamp'>

export type SystemStats = WithRequired<
  OperationResult<'GET /api/client/{id}/system-stats'>,
  'cpu_usage' | 'disk_total' | 'disk_usage' | 'disk_used' | 'memory_total' | 'memory_usage' | 'memory_used'
>
export type ScreenshotData = WithRequired<OperationResult<'GET /api/client/{id}/screenshot'>, 'data' | 'height' | 'timestamp' | 'width'>
export type ShellResult = WithRequired<OperationResult<'POST /api/client/{id}/shell'>, 'exit_code' | 'output'>

type RawServerConfigResponse = OperationResult<'GET /api/config'>
type RawServerConfigInfo = NonNullable<RawServerConfigResponse['server']>
type RawWebConfigInfo = NonNullable<RawServerConfigResponse['web']>
export type ServerConfigResponse = Omit<RawServerConfigResponse, 'server' | 'web'> & {
  server: WithRequired<RawServerConfigInfo, 'bind_addr' | 'bind_port' | 'client_response_timeout_sec' | 'heartbeat_sec' | 'heartbeat_timeout' | 'max_client_proxies' | 'token'>
  web: WithRequired<RawWebConfigInfo, 'bind_port' | 'enabled' | 'password' | 'username'>
}
export type UpdateServerConfigRequest = OperationRequestBody<'PUT /api/config'>
export type ConfigUpdateResult = WithRequired<OperationResult<'PUT /api/config'>, 'status'>

export interface LogEntry {
  ts: number
  level: string
  msg: string
  src: string
}

export interface LogStreamOptions {
  lines?: number
  follow?: boolean
  level?: string
}

type LoginResponse = WithRequired<OperationResult<'POST /api/auth/login'>, 'token'>

export const login = (username: string, password: string) =>
  post<LoginResponse>('/auth/login', { username, password })

export const checkAuth = () => get<OperationResult<'GET /api/auth/check'>>('/auth/check')
export const getServerStatus = () => get<ServerStatus>('/status')

export const getClients = () => get<ClientStatus[]>('/clients')
export const getClient = (id: string) => get<ClientDetail>(`/client/${id}`)
export const addClient = (client: ClientConfig) => post('/clients', client)
export const updateClient = (id: string, client: UpdateClientPayload) => put(`/client/${id}`, client)
export const deleteClient = (id: string) => del(`/client/${id}`)

export const pushConfigToClient = (id: string) => post(`/client/${id}/push`)
export const disconnectClient = (id: string) => post(`/client/${id}/disconnect`)
export const restartClient = (id: string) => post(`/client/${id}/restart`)

export const getVersionInfo = () => get<VersionInfo>('/update/version')
export const checkServerUpdate = () => get<UpdateInfo>('/update/check/server')
export const checkClientUpdate = (os?: string, arch?: string) => {
  const params = new URLSearchParams()
  if (os) params.append('os', os)
  if (arch) params.append('arch', arch)
  const query = params.toString()
  return get<UpdateInfo>(`/update/check/client${query ? `?${query}` : ''}`)
}
export const applyServerUpdate = (downloadUrl: string, restart: boolean = true) =>
  post('/update/apply/server', { download_url: downloadUrl, restart })
export const applyClientUpdate = (clientId: string, downloadUrl: string) =>
  post('/update/apply/client', { client_id: clientId, download_url: downloadUrl })

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
    } catch (error) {
      console.error('Failed to parse log entry', error)
    }
  })

  eventSource.addEventListener('heartbeat', () => {})

  if (onError) {
    eventSource.onerror = onError
  }

  return eventSource
}

export const getTrafficStats = () => get<TrafficStats>('/traffic/stats')
export const getTrafficHourly = () => get<{ records?: TrafficRecord[] }>('/traffic/hourly')

export const getClientSystemStats = (clientId: string) =>
  get<SystemStats>(`/client/${clientId}/system-stats`)

export const getClientScreenshot = (clientId: string, quality?: number) =>
  get<ScreenshotData>(`/client/${clientId}/screenshot${quality ? `?quality=${quality}` : ''}`)

export const executeClientShell = (clientId: string, command: string, timeout?: number) =>
  post<ShellResult>(`/client/${clientId}/shell`, { command, timeout: timeout || 30 })

export const getServerConfig = () => get<ServerConfigResponse>('/config')
export const updateServerConfig = (config: UpdateServerConfigRequest) =>
  put<ConfigUpdateResult>('/config', config)

export const generateInstallCommand = () =>
  post<InstallCommandResponse>('/install/generate')
