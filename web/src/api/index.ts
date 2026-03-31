import { del, get, getToken, post, put } from '../config/axios'
import type {
  ClientListItem,
  ClientResponse,
  ConfigUpdateResponse,
  CreateClientRequest,
  HourlyTrafficResponse,
  InstallCommandResponse,
  LoginResponse,
  ScreenshotResponse,
  ServerConfigResponse,
  StatusResponse,
  SystemStatsResponse,
  TrafficStatsResponse,
  UpdateClientRequest,
  UpdateServerConfigRequest,
  VersionInfo,
} from './types'

export { getToken, removeToken, setToken } from '../config/axios'
export type {
  ApiEnvelope,
  ApplyClientUpdateRequest,
  ApplyServerUpdateRequest,
  CheckUpdateResponse,
  ClientListItem,
  ClientResponse,
  ConfigUpdateResponse,
  CreateClientRequest,
  HourlyTrafficResponse,
  InstallCommandResponse,
  LogEntry,
  LogStreamOptions,
  LoginResponse,
  ProxyRule,
  RemoteControlSocketOptions,
  ScreenshotResponse,
  ServerConfigPart,
  ServerConfigResponse,
  StatusResponse,
  SystemStatsResponse,
  TrafficRecord,
  TrafficStatsResponse,
  UpdateClientRequest,
  UpdateInfo,
  UpdateServerConfigRequest,
  VersionInfo,
  WebConfigPart,
} from './types'

type WithRequired<T, K extends keyof T> = T & { [P in K]-?: NonNullable<T[P]> }

export type ClientConfig = CreateClientRequest
export type UpdateClientPayload = UpdateClientRequest

type RawClientStatus = ClientListItem
export type ClientStatus = WithRequired<RawClientStatus, 'id' | 'online' | 'rule_count'>

type RawClientDetail = ClientResponse
export type ClientDetail = WithRequired<RawClientDetail, 'id' | 'online' | 'rules'>

type RawServerStatus = StatusResponse
type RawServerBindInfo = NonNullable<RawServerStatus['server']>
export type ServerStatus = WithRequired<Omit<RawServerStatus, 'server'>, 'client_count'> & {
  server: WithRequired<RawServerBindInfo, 'bind_addr' | 'bind_port'>
}

export type InstallCommandResult = WithRequired<InstallCommandResponse, 'expires_at' | 'token' | 'tunnel_port'>
export type VersionInfoResult = WithRequired<VersionInfo, 'arch' | 'go_version' | 'os' | 'version'>

type RawTrafficStats = TrafficStatsResponse
type RawTrafficTotals = NonNullable<RawTrafficStats['traffic_24h']>
type TrafficTotals = WithRequired<RawTrafficTotals, 'inbound' | 'outbound'>
export type TrafficStats = Omit<RawTrafficStats, 'traffic_24h' | 'traffic_total'> & {
  traffic_24h: TrafficTotals
  traffic_total: TrafficTotals
}

type RawHourlyTraffic = HourlyTrafficResponse
export type TrafficRecordResult = WithRequired<NonNullable<RawHourlyTraffic['records']>[number], 'inbound' | 'outbound' | 'timestamp'>

export type SystemStats = WithRequired<SystemStatsResponse, 'cpu_usage' | 'disk_total' | 'disk_usage' | 'disk_used' | 'memory_total' | 'memory_usage' | 'memory_used'>
export type ScreenshotData = WithRequired<ScreenshotResponse, 'data' | 'height' | 'timestamp' | 'width'>

type RawServerConfigResponse = ServerConfigResponse
type RawServerConfigInfo = NonNullable<RawServerConfigResponse['server']>
type RawWebConfigInfo = NonNullable<RawServerConfigResponse['web']>
export type ServerConfigResponseResult = Omit<RawServerConfigResponse, 'server' | 'web'> & {
  server: WithRequired<RawServerConfigInfo, 'bind_addr' | 'bind_port' | 'client_response_timeout_sec' | 'heartbeat_sec' | 'heartbeat_timeout' | 'max_client_proxies' | 'token'>
  web: WithRequired<RawWebConfigInfo, 'bind_port' | 'enabled' | 'password' | 'username'>
}
export type ConfigUpdateResult = WithRequired<ConfigUpdateResponse, 'status'>

type LoginResult = WithRequired<LoginResponse, 'token'>

export interface UpdateInfoResult {
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

interface GitHubReleaseAsset {
  browser_download_url?: string
  name?: string
  size?: number
}

interface GitHubRelease {
  assets?: GitHubReleaseAsset[]
  body?: string
  tag_name?: string
}

const GITHUB_API_BASE = 'https://api.github.com'
const GITHUB_REPO_OWNER = 'Flikify'
const GITHUB_REPO_NAME = 'Gotunnel'

export const login = (username: string, password: string) =>
  post<LoginResult>('/auth/login', { username, password })

export const checkAuth = () => get<{ valid?: boolean; username?: string }>('/auth/check')
export const getServerStatus = () => get<ServerStatus>('/runtime/status')
export const getServerInfo = () => get<{ bind_port: number }>('/runtime/status')

export const getClients = () => get<ClientStatus[]>('/clients')
export const getClient = (id: string) => get<ClientDetail>(`/clients/${id}`)
export const addClient = (client: ClientConfig) => post('/clients', client)
export const updateClient = (id: string, client: UpdateClientPayload) => put(`/clients/${id}`, client)
export const deleteClient = (id: string) => del(`/clients/${id}`)

export const pushConfigToClient = (id: string) => post(`/clients/${id}/actions/push-config`)
export const disconnectClient = (id: string) => post(`/clients/${id}/actions/disconnect`)
export const restartClient = (id: string) => post(`/clients/${id}/actions/restart`)

export const getVersionInfo = () => get<VersionInfoResult>('/runtime/version')

const parseVersionParts = (version: string): number[] =>
  version
    .trim()
    .replace(/^[vV]/, '')
    .split(/[^0-9]+/)
    .filter(Boolean)
    .map((part) => Number.parseInt(part, 10))
    .filter((part) => Number.isFinite(part))

const compareVersions = (current: string, latest: string): number => {
  const currentParts = parseVersionParts(current)
  const latestParts = parseVersionParts(latest)
  const maxLength = Math.max(currentParts.length, latestParts.length)

  for (let index = 0; index < maxLength; index += 1) {
    const left = currentParts[index] ?? 0
    const right = latestParts[index] ?? 0
    if (left !== right) {
      return left < right ? -1 : 1
    }
  }

  return 0
}

const fetchLatestRelease = async (): Promise<GitHubRelease> => {
  const response = await fetch(
    `${GITHUB_API_BASE}/repos/${GITHUB_REPO_OWNER}/${GITHUB_REPO_NAME}/releases/latest`,
    {
      headers: {
        Accept: 'application/vnd.github+json',
      },
    }
  )

  if (!response.ok) {
    const body = (await response.text()).slice(0, 200)
    throw new Error(`GitHub Releases request failed: HTTP ${response.status} ${body}`.trim())
  }

  return (await response.json()) as GitHubRelease
}

const findAssetForPlatform = (
  assets: GitHubReleaseAsset[] | undefined,
  component: 'server' | 'client',
  os?: string,
  arch?: string
): GitHubReleaseAsset | undefined => {
  if (!assets?.length || !os || !arch) return undefined

  const prefix = `gotunnel-${component}-`
  const suffix = `-${os}-${arch}`
  return assets.find((asset) => {
    const name = asset.name || ''
    return name.startsWith(prefix) && name.includes(suffix)
  })
}

const buildUpdateInfo = (
  currentVersion: string,
  latestRelease: GitHubRelease,
  asset?: GitHubReleaseAsset
): UpdateInfoResult => {
  const latestVersion = latestRelease.tag_name?.trim()
  if (!latestVersion) {
    throw new Error('GitHub release is missing tag_name')
  }

  return {
    asset_name: asset?.name || '',
    asset_size: asset?.size || 0,
    available: compareVersions(currentVersion, latestVersion) < 0,
    current: currentVersion,
    download_url: asset?.browser_download_url || '',
    latest: latestVersion,
    release_note: latestRelease.body || '',
  }
}

export const checkServerUpdate = async (currentVersion: string, os?: string, arch?: string) => {
  const release = await fetchLatestRelease()
  const asset = findAssetForPlatform(release.assets, 'server', os, arch)
  return { data: buildUpdateInfo(currentVersion, release, asset) }
}

export const checkClientUpdate = async (currentVersion: string, os?: string, arch?: string) => {
  const release = await fetchLatestRelease()
  const asset = findAssetForPlatform(release.assets, 'client', os, arch)
  return { data: buildUpdateInfo(currentVersion, release, asset) }
}

export const getServerUpdateStatus = () =>
  get<ServerUpdateStatus>('/updates/server/status', { timeout: 3000 })

export const applyServerUpdate = (downloadUrl: string, targetVersion: string, restart: boolean = true) =>
  post('/updates/server/actions/apply', { download_url: downloadUrl, target_version: targetVersion, restart })
export const applyClientUpdate = (clientId: string, downloadUrl: string) =>
  post('/updates/clients/actions/apply', { client_id: clientId, download_url: downloadUrl })

export const createLogStream = (
  clientId: string,
  options: import('./types').LogStreamOptions = {},
  onLog: (entry: import('./types').LogEntry) => void,
  onError?: (error: Event) => void
): EventSource => {
  const token = getToken()
  const params = new URLSearchParams()
  if (token) params.append('token', token)
  if (options.lines !== undefined) params.append('lines', String(options.lines))
  if (options.follow !== undefined) params.append('follow', String(options.follow))
  if (options.level) params.append('level', options.level)

  const url = `/api/clients/${clientId}/logs?${params.toString()}`
  const eventSource = new EventSource(url)

  eventSource.addEventListener('log', (event) => {
    try {
      const entry = JSON.parse((event as MessageEvent).data) as import('./types').LogEntry
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

export const getTrafficStats = () => get<TrafficStats>('/runtime/traffic/stats')
export const getTrafficHourly = () => get<{ records?: TrafficRecordResult[] }>('/runtime/traffic/hourly')

export const getClientSystemStats = (clientId: string) =>
  get<SystemStats>(`/clients/${clientId}/system-stats`)

export const getClientScreenshot = (clientId: string, quality?: number) =>
  get<ScreenshotData>(`/clients/${clientId}/screenshot${quality ? `?quality=${quality}` : ''}`)

export const createRemoteControlSocket = (clientId: string, options: import('./types').RemoteControlSocketOptions = {}): WebSocket => {
  const token = getToken()
  if (!token) {
    throw new Error('missing authentication token')
  }

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = new URL(`${protocol}//${window.location.host}/api/clients/${clientId}/remote-control/ws`)
  url.searchParams.set('token', token)
  if (options.quality !== undefined) url.searchParams.set('quality', String(options.quality))
  if (options.maxSide !== undefined) url.searchParams.set('max_side', String(options.maxSide))
  if (options.frameIntervalMs !== undefined) url.searchParams.set('frame_interval_ms', String(options.frameIntervalMs))
  return new WebSocket(url)
}

export const getServerConfig = () => get<ServerConfigResponseResult>('/runtime/config')
export const updateServerConfig = (config: UpdateServerConfigRequest) =>
  put<ConfigUpdateResult>('/runtime/config', config)

export const generateInstallCommand = () =>
  post<InstallCommandResult>('/installations/actions/command')
