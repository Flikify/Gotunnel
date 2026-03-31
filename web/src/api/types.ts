export interface ApiEnvelope<T = unknown> {
  code: number
  data?: T
  message?: string
}

// 请求类型
export interface LoginRequest {
  username: string
  password: string
}

export interface CreateClientRequest {
  id: string
  rules?: ProxyRule[]
}

export interface UpdateClientRequest {
  nickname?: string
  rules?: ProxyRule[]
}

export interface ApplyClientUpdateRequest {
  client_id: string
  download_url: string
}

export interface ApplyServerUpdateRequest {
  download_url: string
  restart?: boolean
}

export interface UpdateServerConfigRequest {
  server?: ServerConfigPart
  web?: WebConfigPart
}

// 响应类型
export interface LoginResponse {
  token?: string
}

export interface TokenCheckResponse {
  username?: string
  valid?: boolean
}

export interface ClientListItem {
  arch?: string
  id?: string
  last_offline_at?: number
  last_ping?: string
  nickname?: string
  online?: boolean
  os?: string
  remote_addr?: string
  rule_count?: number
  version?: string
}

export interface ClientResponse {
  arch?: string
  id?: string
  last_offline_at?: number
  last_ping?: string
  nickname?: string
  online?: boolean
  os?: string
  remote_addr?: string
  rules?: ProxyRule[]
  version?: string
}

export interface ProxyRule {
  auth_enabled?: boolean
  auth_password?: string
  auth_username?: string
  enabled?: boolean
  local_ip?: string
  local_port?: number
  name?: string
  port_status?: string
  remote_port?: number
  type?: string
}

export interface ConfigUpdateResponse {
  applied_runtime_fields?: string[]
  restart_required_fields?: string[]
  status?: string
}

export interface StatusResponse {
  client_count?: number
  server?: ServerStatus
}

export interface ServerStatus {
  bind_addr?: string
  bind_port?: number
}

export interface VersionInfo {
  arch?: string
  build_time?: string
  git_commit?: string
  go_version?: string
  os?: string
  version?: string
}

export interface ServerConfigResponse {
  server?: ServerConfigInfo
  web?: WebConfigInfo
}

export interface ServerConfigInfo {
  bind_addr?: string
  bind_port?: number
  client_response_timeout_sec?: number
  heartbeat_sec?: number
  heartbeat_timeout?: number
  max_client_proxies?: number
  token?: string
}

export interface ServerConfigPart {
  bind_addr?: string
  bind_port?: number
  client_response_timeout_sec?: number
  heartbeat_sec?: number
  heartbeat_timeout?: number
  max_client_proxies?: number
  token?: string
}

export interface WebConfigInfo {
  bind_port?: number
  cdn_prefix?: string
  enabled?: boolean
  password?: string
  username?: string
}

export interface WebConfigPart {
  bind_port?: number
  cdn_prefix?: string
  enabled?: boolean
  password?: string
  username?: string
}

export interface CheckUpdateResponse {
  asset_name?: string
  asset_size?: number
  available?: boolean
  current?: string
  download_url?: string
  latest?: string
  release_note?: string
}

export interface TrafficStatsResponse {
  traffic_24h?: TrafficTotals
  traffic_total?: TrafficTotals
}

export interface TrafficTotals {
  inbound: number
  outbound: number
}

export interface HourlyTrafficResponse {
  records?: TrafficRecord[]
}

export interface TrafficRecord {
  inbound: number
  outbound: number
  timestamp: number
}

export interface SystemStatsResponse {
  cpu_usage?: number
  disk_total?: number
  disk_usage?: number
  disk_used?: number
  memory_total?: number
  memory_usage?: number
  memory_used?: number
}

export interface ScreenshotResponse {
  data?: string
  error?: string
  height?: number
  timestamp?: number
  width?: number
}

export interface InstallCommandResponse {
  expires_at: number
  token: string
  tunnel_port: number
}

export interface RemoteControlSocketOptions {
  quality?: number
  maxSide?: number
  frameIntervalMs?: number
}

export interface UpdateInfo {
  asset_name: string
  asset_size: number
  available: boolean
  current: string
  download_url: string
  latest: string
  release_note: string
}

export interface ServerUpdateStatus {
  current_version: string
  finished_at: number
  message: string
  started_at: number
  state: 'idle' | 'running' | 'restarting' | 'succeeded' | 'failed'
  target_version: string
  updated_at: number
}

export interface HandlerResponse {
  code?: number
  data?: unknown
  message?: string
}
