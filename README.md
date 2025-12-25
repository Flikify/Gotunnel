# GoTunnel

一个轻量级、高性能的内网穿透工具，采用服务端集中化管理模式。

## 项目简介

GoTunnel 是一个类似 frp 的内网穿透解决方案，核心特点是**服务端集中管理配置**。客户端只需提供认证信息即可自动获取映射规则，无需在客户端维护复杂配置。

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

### 可靠性

- **心跳检测** - 可配置的心跳间隔和超时时间，及时发现断线
- **断线重连** - 客户端自动重连机制，网络恢复后自动恢复服务
- **优雅关闭** - 客户端断开时自动释放端口资源

### 安全性

- **Token 认证** - 基于 Token 的身份验证机制
- **客户端白名单** - 仅配置的客户端 ID 可以连接
- **TLS 预留** - 预留 TLS 加密扩展接口

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
./server -c server.yaml
```

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

## 配置系统

服务端使用 YAML 格式的配置文件，集中管理所有客户端的映射规则。

### 配置文件示例

```yaml
# server.yaml
server:
  bind_addr: "0.0.0.0"      # 监听地址
  bind_port: 7000           # 监听端口
  token: "your-secret-token" # 认证 Token
  heartbeat_sec: 30         # 心跳间隔（秒）
  heartbeat_timeout: 90     # 心跳超时（秒）

clients:
  - id: "client-a"
    rules:
      - name: "web"
        local_ip: "127.0.0.1"
        local_port: 80
        remote_port: 8080
      - name: "ssh"
        local_ip: "127.0.0.1"
        local_port: 22
        remote_port: 2222

  - id: "client-b"
    rules:
      - name: "mysql"
        local_ip: "127.0.0.1"
        local_port: 3306
        remote_port: 13306
```

### 配置参数说明

**Server 配置：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `bind_addr` | string | - | 服务端监听地址 |
| `bind_port` | int | - | 服务端监听端口 |
| `token` | string | - | 客户端认证 Token |
| `heartbeat_sec` | int | 30 | 心跳发送间隔（秒） |
| `heartbeat_timeout` | int | 90 | 心跳超时时间（秒） |

**Rule 配置：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `name` | string | 规则名称（用于日志标识） |
| `local_ip` | string | 客户端本地服务 IP |
| `local_port` | int | 客户端本地服务端口 |
| `remote_port` | int | 服务端对外暴露端口 |

## 项目结构

```
GoTunnel/
├── cmd/
│   ├── server/main.go    # 服务端入口
│   └── client/main.go    # 客户端入口
├── pkg/
│   ├── protocol/         # 通信协议定义
│   ├── config/           # 配置文件解析
│   ├── tunnel/           # 核心隧道逻辑
│   └── utils/            # 工具函数
├── server.yaml           # 配置示例
└── go.mod
```

## 许可证

本项目采用 [MIT License](LICENSE) 开源许可证。

