# GoTunnel

一个轻量级、高性能的内网穿透工具，采用服务端集中化管理模式，支持 TLS 加密通信。

## 项目简介

GoTunnel 是一个类似 frp 的内网穿透解决方案，核心特点是**服务端集中管理配置**和**零配置 TLS 加密**。客户端只需提供认证信息即可自动获取映射规则，无需在客户端维护复杂配置。

### 架构设计

```
┌──────────────┐                    ┌──────────────────────┐
│   Client     │   Control Channel  │       Server         │
│              │  ◄────────────────►│                      │
│  ┌────────┐  │    (Yamux Mux)     │  ┌────────────────┐  │
│  │ Agent  │  │                    │  │ Control Manager│  │
│  └────────┘  │                    │  └────────────────┘  │
│       │      │                    │          │           │
│       ▼      │    Data Streams    │          ▼           │
│  ┌────────┐  │  ◄────────────────►│  ┌────────────────┐  │
│  │ Proxy  │  │                    │  │ Proxy Listener │  │
│  └────────┘  │                    │  │  :8080,:9090   │  │
│       │      │                    │  └────────────────┘  │
│       ▼      │                    │          ▲           │
│  ┌────────┐  │                    │          │           │
│  │Local:80│  │                    │   External Users     │
│  └────────┘  │                    │                      │
└──────────────┘                    └──────────────────────┘
```

## 功能特性

### 核心功能

- **服务端集中管理** - 所有客户端的映射规则由服务端统一配置，客户端零配置
- **多路复用** - 基于 Yamux 实现控制通道与数据通道分离，高效复用单一 TCP 连接
- **多客户端支持** - 支持多个客户端同时连接，每个客户端独立的映射规则
- **端口冲突检测** - 自动检测系统端口占用和客户端间端口冲突
- **SOCKS5/HTTP 代理** - 支持通过客户端网络访问任意网站

### 安全性

- **TLS 加密** - 默认启用 TLS 加密，证书自动生成，零配置
- **Token 认证** - 基于 Token 的身份验证机制
- **客户端白名单** - 仅配置的客户端 ID 可以连接

### 可靠性

- **心跳检测** - 可配置的心跳间隔和超时时间，及时发现断线
- **断线重连** - 客户端自动重连机制，网络恢复后自动恢复服务
- **优雅关闭** - 客户端断开时自动释放端口资源

### Web 管理

- **Web 控制台** - 内置 Web 管理界面，可视化管理客户端和规则
- **实时状态** - 查看客户端在线状态
- **动态配置** - 在线添加/修改客户端规则

## 快速开始

### 安装

**从源码编译：**

```bash
git clone https://github.com/your-repo/gotunnel.git
cd gotunnel
go build -o server ./cmd/server
go build -o client ./cmd/client
```

**下载预编译二进制：**

从 Releases 页面下载对应平台的二进制文件。

### 服务端启动

```bash
# 零配置启动（自动生成 Token，启用 TLS 和 Web 控制台）
./server

# 或指定配置文件
./server -c server.yaml
```

首次启动会自动：
- 生成随机 Token（打印在日志中）
- 启用 TLS 加密（证书在内存中自动生成）
- 启动 Web 控制台（默认 http://localhost:7500）
- 创建 SQLite 数据库存储客户端配置

### 客户端启动

```bash
./client -s <服务器IP>:7000 -t <Token> -id <客户端ID>
```

**参数说明：**

| 参数 | 说明 | 必填 |
|------|------|------|
| `-s` | 服务器地址 (ip:port) | 是 |
| `-t` | 认证 Token | 是 |
| `-id` | 客户端 ID（需与服务端配置匹配） | 否（自动生成） |
| `-no-tls` | 禁用 TLS 加密 | 否 |

## 配置系统

服务端使用 YAML 配置文件 + SQLite 数据库管理。YAML 配置服务端参数，SQLite 存储客户端规则。

### 配置文件示例

```yaml
# server.yaml
server:
  bind_addr: "0.0.0.0"      # 监听地址
  bind_port: 7000           # 监听端口
  token: "your-secret-token" # 认证 Token（不配置则自动生成）
  heartbeat_sec: 30         # 心跳间隔（秒）
  heartbeat_timeout: 90     # 心跳超时（秒）
  db_path: "gotunnel.db"    # 数据库路径
  tls_disabled: false       # 是否禁用 TLS（默认启用）

web:
  enabled: true             # 启用 Web 控制台
  bind_addr: "0.0.0.0"
  bind_port: 7500
  username: "admin"         # 可选，设置后启用认证
  password: "password"
```

### 配置参数说明

**Server 配置：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `bind_addr` | string | 0.0.0.0 | 服务端监听地址 |
| `bind_port` | int | 7000 | 服务端监听端口 |
| `token` | string | 自动生成 | 客户端认证 Token |
| `heartbeat_sec` | int | 30 | 心跳发送间隔（秒） |
| `heartbeat_timeout` | int | 90 | 心跳超时时间（秒） |
| `db_path` | string | gotunnel.db | SQLite 数据库路径 |
| `tls_disabled` | bool | false | 是否禁用 TLS |

**Web 配置：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | true | 是否启用 Web 控制台 |
| `bind_addr` | string | 0.0.0.0 | Web 监听地址 |
| `bind_port` | int | 7500 | Web 监听端口 |
| `username` | string | - | 认证用户名（可选） |
| `password` | string | - | 认证密码（可选） |

## 代理规则类型

通过 Web 控制台配置客户端规则时，支持以下类型：

| 类型 | 说明 | 示例用途 |
|------|------|----------|
| `tcp` | TCP 端口转发（默认） | SSH、MySQL、Web 服务 |
| `socks5` | SOCKS5 代理 | 通过客户端网络访问任意地址 |
| `http` | HTTP 代理 | 通过客户端网络访问 HTTP/HTTPS |

**规则配置示例（通过 Web API）：**

```json
{
  "id": "client-a",
  "rules": [
    {"name": "web", "type": "tcp", "local_ip": "127.0.0.1", "local_port": 80, "remote_port": 8080},
    {"name": "socks5-proxy", "type": "socks5", "remote_port": 1080},
    {"name": "http-proxy", "type": "http", "remote_port": 8888}
  ]
}
```

## 项目结构

```
GoTunnel/
├── cmd/
│   ├── server/main.go       # 服务端入口
│   └── client/main.go       # 客户端入口
├── internal/
│   ├── server/
│   │   ├── tunnel/          # 隧道服务
│   │   ├── config/          # 配置管理
│   │   ├── db/              # 数据库存储
│   │   ├── app/             # Web 服务
│   │   └── router/          # API 路由
│   └── client/
│       └── tunnel/          # 客户端隧道
├── pkg/
│   ├── protocol/            # 通信协议
│   ├── crypto/              # TLS 加密
│   ├── proxy/               # SOCKS5/HTTP 代理
│   ├── relay/               # 数据转发
│   └── utils/               # 工具函数
└── go.mod
```

## 许可证

本项目采用 [MIT License](LICENSE) 开源许可证。

