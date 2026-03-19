// 代理规则
export interface ProxyRule {
  name: string
  local_ip: string
  local_port: number
  remote_port: number
  type?: string
  enabled?: boolean
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
  remote_addr?: string
  rule_count: number
  os?: string
  arch?: string
}

// 客户端详情
export interface ClientDetail {
  id: string
  nickname?: string
  rules: ProxyRule[]
  online: boolean
  last_ping?: string
  remote_addr?: string
  os?: string
  arch?: string
  version?: string
}

// 服务器状态
export interface ServerStatus {
  server: {
    bind_addr: string
    bind_port: number
  }
  client_count: number
}

// 日志条目
export interface LogEntry {
  ts: number      // Unix 时间戳 (毫秒)
  level: string   // 日志级别: debug, info, warn, error
  msg: string     // 日志消息
  src: string     // 来源: client
}

// 日志流选项
export interface LogStreamOptions {
  lines?: number   // 初始日志行数
  follow?: boolean // 是否持续推送
  level?: string   // 日志级别过滤
}

// 安装命令响应
export interface InstallCommandResponse {
  token: string
  expires_at: number
  tunnel_port: number
}
