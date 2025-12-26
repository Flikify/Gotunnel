// 代理规则
export interface ProxyRule {
  name: string
  local_ip: string
  local_port: number
  remote_port: number
  type?: string
}

// 客户端配置
export interface ClientConfig {
  id: string
  nickname?: string
  rules: ProxyRule[]
}

// 客户端状态
export interface ClientStatus {
  id: string
  nickname?: string
  online: boolean
  last_ping?: string
  rule_count: number
}

// 客户端详情
export interface ClientDetail {
  id: string
  nickname?: string
  rules: ProxyRule[]
  online: boolean
  last_ping?: string
}

// 服务器状态
export interface ServerStatus {
  server: {
    bind_addr: string
    bind_port: number
  }
  client_count: number
}

// 插件类型
export const PluginType = {
  Proxy: 'proxy',
  App: 'app',
  Service: 'service',
  Tool: 'tool'
} as const

export type PluginTypeValue = typeof PluginType[keyof typeof PluginType]

// 插件信息
export interface PluginInfo {
  name: string
  version: string
  type: string
  description: string
  source: string
  enabled: boolean
}
