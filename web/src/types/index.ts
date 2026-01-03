// 代理规则
export interface ProxyRule {
  name: string
  local_ip: string
  local_port: number
  remote_port: number
  type?: string
  enabled?: boolean
  plugin_config?: Record<string, string>
}

// 客户端已安装的插件
export interface ClientPlugin {
  name: string
  version: string
  enabled: boolean
  running: boolean
  config?: Record<string, string>
}

// 插件配置字段
export interface ConfigField {
  key: string
  label: string
  type: 'string' | 'number' | 'bool' | 'select' | 'password'
  default?: string
  required?: boolean
  options?: string[]
  description?: string
}

// 规则表单模式
export interface RuleSchema {
  needs_local_addr: boolean
  extra_fields?: ConfigField[]
}

// 插件配置响应
export interface PluginConfigResponse {
  plugin_name: string
  schema: ConfigField[]
  config: Record<string, string>
}

// 客户端配置
export interface ClientConfig {
  id: string
  nickname?: string
  rules: ProxyRule[]
  plugins?: ClientPlugin[]
}

// 客户端状态
export interface ClientStatus {
  id: string
  nickname?: string
  online: boolean
  last_ping?: string
  remote_addr?: string
  rule_count: number
}

// 客户端详情
export interface ClientDetail {
  id: string
  nickname?: string
  rules: ProxyRule[]
  plugins?: ClientPlugin[]
  online: boolean
  last_ping?: string
  remote_addr?: string
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
  icon?: string
  enabled: boolean
  rule_schema?: RuleSchema
}

// 扩展商店插件信息
export interface StorePluginInfo {
  name: string
  version: string
  type: string
  description: string
  author: string
  icon?: string
  download_url?: string
  signature_url?: string
}

// JS 插件信息
export interface JSPlugin {
  name: string
  source: string
  signature?: string
  description: string
  author: string
  version?: string
  auto_push: string[]
  config: Record<string, string>
  auto_start: boolean
  enabled: boolean
}

// 规则配置模式集合
export type RuleSchemasMap = Record<string, RuleSchema>

// 日志条目
export interface LogEntry {
  ts: number      // Unix 时间戳 (毫秒)
  level: string   // 日志级别: debug, info, warn, error
  msg: string     // 日志消息
  src: string     // 来源: client, plugin:<name>
}

// 日志流选项
export interface LogStreamOptions {
  lines?: number   // 初始日志行数
  follow?: boolean // 是否持续推送
  level?: string   // 日志级别过滤
}
