// 代理规则
export interface ProxyRule {
  name: string
  local_ip: string
  local_port: number
  remote_port: number
}

// 客户端配置
export interface ClientConfig {
  id: string
  rules: ProxyRule[]
}

// 客户端状态
export interface ClientStatus {
  id: string
  online: boolean
  last_ping?: string
  rule_count: number
}

// 客户端详情
export interface ClientDetail {
  id: string
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
