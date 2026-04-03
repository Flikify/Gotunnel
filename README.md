# GoTunnel

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

一个轻量级、高性能的内网穿透工具，采用服务端集中化管理模式，支持 TLS 加密通信。

## 项目简介

GoTunnel 是一个类似 frp 的内网穿透解决方案，核心特点是**服务端集中管理配置**和**零配置 TLS 加密**。客户端只需提供认证信息即可自动获取映射规则，无需在客户端维护复杂配置。

### 与 frp 的主要区别

| 特性 | GoTunnel | frp |
|------|----------|-----|
| 配置管理 | 服务端集中管理 | 客户端各自配置 |
| TLS 证书 | 自动生成，零配置 | 需手动配置 |
| 管理界面 | 内置 Web 控制台 (naive-ui) | 需额外部署 Dashboard |
| 客户端部署 | 仅需 2 个参数 | 需配置文件 |
| 客户端 ID | 自动根据设备标识计算 | 需手动配置 |

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
- **自动更新** - 服务端和客户端支持从 Web 界面一键更新

### 安全性

- **TLS 加密** - 默认启用 TLS 加密，证书自动生成，零配置
- **TOFU 证书验证** - 首次连接信任 (Trust On First Use)，防止中间人攻击
- **Token 认证** - 基于 Token 的身份验证机制
- **强制 Web 认证** - Web 控制台强制启用 JWT 认证
- **安全审计日志** - 记录所有认证事件和安全相关操作
- **连接数限制** - 防止资源耗尽攻击 (默认 10000 连接上限)
- **客户端 ID 验证** - 严格的 ID 格式校验，防止注入攻击

### 可靠性

- **心跳检测** - 可配置的心跳间隔和超时时间，及时发现断线
- **断线重连** - 客户端自动重连机制，网络恢复后自动恢复服务
- **优雅关闭** - 支持 SIGINT/SIGTERM 信号，安全关闭所有连接
- **资源自动释放** - 客户端断开时自动释放端口资源

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
# 最简启动（ID 由客户端根据设备标识自动计算）
./client -s <服务器IP>:7000 -t <Token>

```

**参数说明：**

| 参数 | 说明 | 必填 |
|------|------|------|
| `-s` | 服务器地址 (ip:port) | 是 |
| `-t` | 认证 Token | 是 |

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

### 内置类型

| 类型 | 说明 | 示例用途 |
|------|------|----------|
| `tcp` | TCP 端口转发（默认） | SSH、MySQL、Web 服务 |
| `udp` | UDP 端口转发 | DNS、游戏服务器、VoIP |
| `http` | HTTP 代理 | 通过客户端网络访问 HTTP/HTTPS |
| `https` | HTTPS 代理 | 同 HTTP，支持 CONNECT 方法 |

| `socks5` | SOCKS5 代理 | 通过客户端网络访问任意地址 |

**规则配置示例（通过 Web API）：**

```json
{
  "id": "client-a",
  "nickname": "办公室电脑",
  "rules": [
    {"name": "web", "type": "tcp", "local_ip": "127.0.0.1", "local_port": 80, "remote_port": 8080},
    {"name": "dns", "type": "udp", "local_ip": "127.0.0.1", "local_port": 53, "remote_port": 5353},
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
│   ├── core/
│   │   ├── client/          # 共享客户端聚合模型
│   │   ├── domain/          # 共享领域别名聚合
│   │   └── rule/            # 共享代理规则模型
│   ├── client/
│   │   ├── app/             # 客户端生命周期应用层
│   │   └── tunnel/          # 客户端运行时
│   └── server/
│       ├── app/             # Web 服务
│       ├── bootstrap/       # 服务端启动装配
│       ├── config/          # 配置管理
│       ├── http/            # HTTP 路由与处理器
│       ├── service/         # 应用服务
│       ├── storage/sqlite/  # SQLite 存储适配层
│       ├── runtime/         # 隧道运行时
│       └── updateapp/       # 服务端更新编排
├── mobile/
│   └── gotunnelmobile/      # gomobile 适配层
├── android/                 # Android 宿主应用
├── pkg/
│   ├── auth/                # JWT 认证
│   ├── crypto/              # TLS 加密
│   ├── protocol/            # 通信协议
│   ├── proxy/               # 代理服务器
│   ├── relay/               # 数据转发
│   ├── security/            # 安全审计辅助
│   ├── utils/               # 工具函数
│   ├── update/              # 共享更新逻辑 (下载、解压)
│   └── version/             # 版本信息和更新检查
├── web/                     # Vue 3 前端
├── scripts/                 # 构建脚本
│   ├── build.sh             # Linux/macOS 构建脚本
│   └── build.ps1            # Windows 构建脚本
└── go.mod
```

> 说明：当前仓库不包含独立的 JS 插件系统，`plugin/` 相关目录和 `/api/plugins` 接口已从 README 中移除，以免与实际实现混淆。

## Web API

Web 控制台提供 RESTful API 用于管理客户端、配置、更新和运行时操作。配置了 `username` 和 `password` 后，所有 `/api/*` 请求都需要 JWT 认证。

### 认证

```bash
# 登录获取 Token
POST /api/auth/login
Content-Type: application/json
{"username": "admin", "password": "password"}

# 响应
{"token": "eyJhbGciOiJIUzI1NiIs..."}

# 后续请求携带 Token
Authorization: Bearer <token>
```

### API 列表

#### 认证

- `POST /api/auth/login`
- `GET /api/auth/check`

#### 状态与版本

- `GET /api/runtime/status`
- `GET /api/runtime/version`

#### 客户端管理

- `GET /api/clients`
- `POST /api/clients`
- `GET /api/clients/{id}`
- `PUT /api/clients/{id}`
- `DELETE /api/clients/{id}`

#### 客户端控制与远程运维

- `POST /api/clients/{id}/actions/push-config`
- `POST /api/clients/{id}/actions/disconnect`
- `POST /api/clients/{id}/actions/restart`
- `GET /api/clients/{id}/logs`
- `GET /api/clients/{id}/system-stats`
- `GET /api/clients/{id}/screenshot`

#### 配置管理

- `GET /api/runtime/config`
- `PUT /api/runtime/config`

`PUT /api/runtime/config` 会把配置持久化到 YAML，并返回两类结果：已即时生效的运行时字段，以及需要重启服务后才会生效的字段。当前版本不再暴露独立的热重载入口。

#### 更新管理

- `GET /api/updates/server`
- `GET /api/updates/clients/latest`
- `POST /api/updates/server/actions/apply`
- `POST /api/updates/clients/actions/apply`

#### 流量统计与安装

- `GET /api/runtime/traffic/stats`
- `GET /api/runtime/traffic/hourly`
- `POST /api/installations/actions/command`

#### 安装脚本

- `GET /install.sh`（兼容保留）
- `GET /install.ps1`（兼容保留）

推荐直接使用 GitHub 中的安装脚本：

- `https://raw.githubusercontent.com/Flikify/Gotunnel/main/scripts/install.sh`
- `https://raw.githubusercontent.com/Flikify/Gotunnel/main/scripts/install.ps1`

PowerShell 中建议使用单引号包裹 `-Command` 的脚本体。下面这个版本更短，也能避免当前 shell 预先展开 `$env:TEMP`：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -Command 'iwr "https://raw.githubusercontent.com/Flikify/Gotunnel/main/scripts/install.ps1" -OutFile "$env:TEMP\gotunnel-install.ps1"; & "$env:TEMP\gotunnel-install.ps1" -Server "127.0.0.1:7000" -Token "replace-with-token"'
```

Linux / macOS 也可以使用更短的一键命令：

```bash
f=$(mktemp);curl -fsSL 'https://raw.githubusercontent.com/Flikify/Gotunnel/main/scripts/install.sh' -o $f&&bash $f -s '127.0.0.1:7000' -t 'replace-with-token';rm -f $f
```

当前安装脚本行为：

- Windows：注册为 Windows Service，开机自启，失败自动重启。
- Linux：注册为 `systemd` 服务 `gotunnel-client.service`，开机自启，失败自动重启。
- macOS：注册为 `launchd` daemon `com.gotunnel.client`，开机自启，异常退出自动拉起。

注意：

- Windows 客户端如果以 Windows Service 运行，会由服务进程保活隧道连接，并在检测到活动用户会话时拉起一个该会话内的 desktop helper 来处理截图和远控画面采集。
- 如果当前机器没有已登录的活动用户会话，或者桌面被锁定 / 切到 UAC secure desktop，截图和远控画面仍然会不可用。

## 使用场景

### 场景一：暴露内网 Web 服务

```bash
# 服务端配置客户端规则（通过 Web 控制台或 API）
curl -X POST http://server:7500/api/clients \
  -H "Content-Type: application/json" \
  -d '{"id":"home","rules":[{"name":"web","type":"tcp","local_ip":"127.0.0.1","local_port":80,"remote_port":8080}]}'

# 客户端连接
./client -s server:7000 -t <token>

# 访问：http://server:8080 -> 内网 127.0.0.1:80
```

### 场景二：SOCKS5 代理访问内网

```bash
# 配置 SOCKS5 代理规则
{"name":"proxy","type":"socks5","remote_port":1080}

# 使用代理
curl --socks5 server:1080 http://internal-service/
```

## 常见问题

**Q: 客户端连接后如何设置昵称？**

A: 在 Web 控制台点击客户端详情，进入编辑模式即可设置昵称。

**Q: 如何禁用 TLS？**

A: 客户端命令行默认使用 TLS；如需兼容旧的非 TLS 部署，请改用客户端配置文件中的 `no_tls: true`。

**Q: 端口被占用怎么办？**

A: 服务端会自动检测端口冲突，请检查日志并更换端口。

**Q: 客户端 ID 是如何分配的？**

A: 客户端会把系统机器 ID、全部可用 MAC、主机名和网卡名等稳定标识组合后再进行哈希，得到固定客户端 ID；服务端不再为客户端分配或修正 ID。

**Q: 如何更新服务端/客户端？**

A: 在 Web 控制台的"更新"页面，可以检查并应用更新。服务端/客户端会自动从 Release 下载压缩包、解压并重启。

## 构建

使用构建脚本可以一键构建前后端：

**Linux/macOS:**

```bash
# 构建当前平台
./scripts/build.sh current

# 构建所有平台
./scripts/build.sh all

# 仅构建 Web UI
./scripts/build.sh web

# 清理构建产物
./scripts/build.sh clean

# 指定版本号
VERSION=1.0.0 ./scripts/build.sh all
```

**Windows (PowerShell):**

```powershell
# 构建当前平台
.\scripts\build.ps1 current

# 构建所有平台
.\scripts\build.ps1 all

# 仅构建 Web UI
.\scripts\build.ps1 web

# 清理构建产物
.\scripts\build.ps1 clean

# 指定版本号
$env:VERSION="1.0.0"; .\scripts\build.ps1 all
```

构建产物输出到 `build/<os>_<arch>/` 目录。

## 架构时序图

### 1. 连接建立阶段

```
┌────────┐          ┌────────┐          ┌──────────┐
│ Client │          │ Server │          │ Database │
└───┬────┘          └───┬────┘          └────┬─────┘
    │                   │                    │
    │ 1. TCP/TLS Connect│                    │
    │──────────────────>│                    │
    │                   │                    │
    │ 2. AuthRequest    │                    │
    │   {token, id?}    │                    │
    │──────────────────>│                    │
    │                   │ 3. 验证 Token       │
    │                   │ 4. 查询/创建客户端  │
    │                   │───────────────────>│
    │                   │<───────────────────│
    │                   │                    │
    │ 5. AuthResponse   │                    │
    │   {ok, client_id} │                    │
    │<──────────────────│                    │
    │                   │                    │
    │ 6. Yamux Session  │                    │
    │<═════════════════>│                    │
    │                   │                    │
    │ 7. ProxyConfig    │                    │
    │   {rules[]}       │                    │
    │<──────────────────│                    │
    │                   │                    │
```

### 2. TCP 代理数据流

```
┌──────────┐    ┌────────┐    ┌────────┐    ┌───────────────┐
│ External │    │ Server │    │ Client │    │ Local Service │
└────┬─────┘    └───┬────┘    └───┬────┘    └───────┬───────┘
     │              │             │                 │
     │ 1. Connect   │             │                 │
     │  :remote     │             │                 │
     │─────────────>│             │                 │
     │              │             │                 │
     │              │ 2. Yamux    │                 │
     │              │   Stream    │                 │
     │              │────────────>│                 │
     │              │             │                 │
     │              │ 3. NewProxy │                 │
     │              │  {port}     │                 │
     │              │────────────>│                 │
     │              │             │                 │
     │              │             │ 4. Connect      │
     │              │             │   local:port    │
     │              │             │────────────────>│
     │              │             │                 │
     │ 5. Relay (双向数据转发)    │                 │
     │<════════════>│<═══════════>│<═══════════════>│
     │              │             │                 │
```

### 3. SOCKS5/HTTP 代理数据流

```
┌──────────┐    ┌────────┐    ┌────────┐    ┌─────────────┐
│ External │    │ Server │    │ Client │    │ Target Host │
└────┬─────┘    └───┬────┘    └───┬────┘    └──────┬──────┘
     │              │             │                │
     │ 1. SOCKS5    │             │                │
     │   Handshake  │             │                │
     │─────────────>│             │                │
     │              │             │                │
     │              │ 2. Proxy    │                │
     │              │   Connect   │                │
     │              │  {target}   │                │
     │              │────────────>│                │
     │              │             │                │
     │              │             │ 3. Dial target │
     │              │             │───────────────>│
     │              │             │                │
     │              │ 4. Result   │                │
     │              │<────────────│                │
     │              │             │                │
     │ 5. Relay (双向数据转发)    │                │
     │<════════════>│<═══════════>│<══════════════>│
     │              │             │                │
```

### 4. 心跳保活机制

```
┌────────┐          ┌────────┐
│ Client │          │ Server │
└───┬────┘          └───┬────┘
    │                   │
    │                   │ 1. Ticker (30s)
    │                   │
    │ 2. Heartbeat      │
    │<──────────────────│
    │                   │
    │ 3. HeartbeatAck   │
    │──────────────────>│
    │                   │
    │                   │ 4. 更新 LastPing
    │                   │
    │    ... 循环 ...   │
    │                   │
    │                   │ 5. 超时检测 (90s)
    │                   │   无响应则断开
    │                   │
```

### 核心组件说明

| 组件 | 职责 |
|------|------|
| Client | 连接服务端、转发本地服务、响应心跳 |
| Server | 认证、管理会话、监听端口、路由流量 |
| Yamux | 单连接多路复用，承载控制和数据通道 |
| Plugin | 处理 SOCKS5/HTTP 等代理协议 |
| PortManager | 端口分配与释放管理 |
| Database | 持久化客户端配置和规则 |

## 许可证

本项目采用 [MIT License](LICENSE) 开源许可证。
