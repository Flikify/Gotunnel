# GoTunnel 重构计划

## 概述

本次重构包含三个主要目标：
1. 移除 WASM 支持，只保留 JS 插件系统
2. 优化 Web 界面，支持协议动态配置和 JS 插件管理
3. 实现动态重启客户端和插件功能

---

## 第一部分：移除 WASM，简化插件系统

### 1.1 需要删除的文件/目录
- `pkg/plugin/wasm/` - WASM 运行时目录（如果存在）

### 1.2 需要修改的文件

#### 数据库层 (`internal/server/db/`)
- **interface.go**: 移除 `PluginStore` 接口中的 `GetPluginWASM` 方法
- **sqlite.go**:
  - 移除 `plugins` 表（WASM 插件表）
  - 移除相关的 CRUD 方法
  - 保留 `js_plugins` 表

#### 插件类型 (`pkg/plugin/types.go`)
- 移除 `PluginSource` 中的 `"wasm"` 选项，只保留 `"builtin"` 和 `"script"`

#### 依赖清理
- 检查 `go.mod` 是否有 wazero 依赖，如有则移除

---

## 第二部分：优化 Web 界面

### 2.1 协议动态配置

#### 后端修改

##### A. 扩展 ConfigField 类型 (`pkg/plugin/types.go`)
```go
type ConfigField struct {
    Key         string   `json:"key"`
    Label       string   `json:"label"`
    Type        string   `json:"type"` // string, number, bool, select, password
    Default     string   `json:"default,omitempty"`
    Required    bool     `json:"required,omitempty"`
    Options     []string `json:"options,omitempty"`
    Description string   `json:"description,omitempty"`
}

type RuleSchema struct {
    NeedsLocalAddr bool          `json:"needs_local_addr"`
    ExtraFields    []ConfigField `json:"extra_fields"`
}
```

##### B. 内置协议配置模式
为 SOCKS5 和 HTTP 代理添加认证配置字段：

```go
// SOCKS5 配置模式
var Socks5Schema = RuleSchema{
    NeedsLocalAddr: false,
    ExtraFields: []ConfigField{
        {Key: "auth_enabled", Label: "启用认证", Type: "bool", Default: "false"},
        {Key: "username", Label: "用户名", Type: "string"},
        {Key: "password", Label: "密码", Type: "password"},
    },
}

// HTTP 代理配置模式
var HTTPProxySchema = RuleSchema{
    NeedsLocalAddr: false,
    ExtraFields: []ConfigField{
        {Key: "auth_enabled", Label: "启用认证", Type: "bool", Default: "false"},
        {Key: "username", Label: "用户名", Type: "string"},
        {Key: "password", Label: "密码", Type: "password"},
    },
}
```

##### C. API 端点 (`internal/server/router/api.go`)
- **GET `/api/rule-schemas`**: 返回所有协议类型的配置模式
  - 内置类型 (tcp, udp, http, https) 的模式
  - 已注册插件的模式（从插件 Metadata 获取）

#### 前端修改 (`web/src/views/ClientView.vue`)
- 页面加载时获取 rule-schemas
- 根据选择的协议类型动态渲染额外配置字段
- 支持的字段类型：string, number, bool, select, password

### 2.2 JS 插件管理界面优化

#### 后端修改

##### A. 扩展 JSPlugin 结构 (`internal/server/db/interface.go`)
```go
type JSPlugin struct {
    Name        string              `json:"name"`
    Source      string              `json:"source"`      // JS 源码
    Signature   string              `json:"signature"`   // 官方签名
    Description string              `json:"description"`
    Author      string              `json:"author"`
    Version     string              `json:"version"`
    AutoPush    []string            `json:"auto_push"`   // 自动推送客户端列表
    Config      map[string]string   `json:"config"`      // 插件运行时配置
    ConfigSchema []ConfigField      `json:"config_schema"` // 配置字段定义
    AutoStart   bool                `json:"auto_start"`
    Enabled     bool                `json:"enabled"`
    CreatedAt   time.Time           `json:"created_at"`
    UpdatedAt   time.Time           `json:"updated_at"`
}
```

##### B. 新增/修改 API 端点
- **GET `/api/js-plugins`**: 获取所有 JS 插件列表（包含配置模式）
- **GET `/api/js-plugin/{name}`**: 获取单个插件详情
- **PUT `/api/js-plugin/{name}/config`**: 更新插件运行时配置
- **POST `/api/js-plugin/{name}/push/{clientId}`**: 推送插件到指定客户端
- **POST `/api/js-plugin/{name}/reload`**: 重新加载插件（重新解析源码）

#### 前端修改 (`web/src/views/PluginsView.vue`)

##### A. 重新设计 JS 插件 Tab
```
┌─────────────────────────────────────────────────────────────┐
│  已安装的 JS 插件                                            │
├─────────────────────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ 📦 socks5-auth                              v1.0.0     │ │
│ │ 带认证的 SOCKS5 代理                                    │ │
│ │ 作者: official                                         │ │
│ │ ────────────────────────────────────────────────────── │ │
│ │ 状态: ✅ 启用    自动启动: ✅                           │ │
│ │ 推送目标: 所有客户端                                    │ │
│ │ ────────────────────────────────────────────────────── │ │
│ │ [⚙️ 配置] [🔄 重载] [📤 推送] [🗑️ 删除]                │ │
│ └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

##### B. 插件配置模态框
- 动态渲染 ConfigSchema 定义的字段
- 支持 string, number, bool, select, password 类型
- 保存后可选择是否同步到已连接的客户端

##### C. 推送配置
- 选择目标客户端（支持多选）
- 选择是否自动启动
- 显示推送结果

---

## 第三部分：动态重启功能

### 3.1 协议消息定义 (`pkg/protocol/message.go`)

```go
// 已有消息类型
const (
    MsgTypeClientPluginStart  = 40  // 启动插件
    MsgTypeClientPluginStop   = 41  // 停止插件 (需实现)
    MsgTypeClientPluginStatus = 42  // 插件状态
    MsgTypeClientPluginConn   = 43  // 插件连接

    // 新增消息类型
    MsgTypeClientRestart       = 44  // 客户端重启
    MsgTypePluginConfigUpdate  = 45  // 插件配置更新
)

// 新增请求/响应结构
type ClientPluginStopRequest struct {
    PluginName string `json:"plugin_name"`
    RuleName   string `json:"rule_name"`
}

type ClientRestartRequest struct {
    Reason string `json:"reason,omitempty"`
}

type PluginConfigUpdateRequest struct {
    PluginName string            `json:"plugin_name"`
    RuleName   string            `json:"rule_name"`
    Config     map[string]string `json:"config"`
}
```

### 3.2 客户端实现 (`internal/client/tunnel/client.go`)

#### A. 实现插件停止处理
```go
func (c *Client) handleClientPluginStop(stream net.Conn, msg *protocol.Message) {
    defer stream.Close()

    var req protocol.ClientPluginStopRequest
    if err := msg.ParsePayload(&req); err != nil {
        return
    }

    key := req.PluginName + ":" + req.RuleName

    c.pluginMu.Lock()
    if handler, ok := c.runningPlugins[key]; ok {
        handler.Stop()
        delete(c.runningPlugins, key)
    }
    c.pluginMu.Unlock()

    // 发送确认
    resp := protocol.ClientPluginStatusResponse{
        PluginName: req.PluginName,
        RuleName:   req.RuleName,
        Running:    false,
    }
    respMsg, _ := protocol.NewMessage(protocol.MsgTypeClientPluginStatus, resp)
    protocol.WriteMessage(stream, respMsg)
}
```

#### B. 实现配置热更新
```go
func (c *Client) handlePluginConfigUpdate(stream net.Conn, msg *protocol.Message) {
    // 更新运行中插件的配置（如果插件支持热更新）
}
```

#### C. 实现客户端优雅重启
```go
func (c *Client) handleClientRestart(stream net.Conn, msg *protocol.Message) {
    // 1. 停止所有运行中的插件
    // 2. 关闭当前会话
    // 3. 触发重连（Run() 循环会自动处理）
}
```

### 3.3 服务端实现 (`internal/server/tunnel/server.go`)

```go
// 停止客户端插件
func (s *Server) StopClientPlugin(clientID, pluginName, ruleName string) error

// 重启客户端插件
func (s *Server) RestartClientPlugin(clientID, pluginName, ruleName string) error

// 更新插件配置
func (s *Server) UpdateClientPluginConfig(clientID, pluginName, ruleName string, config map[string]string) error

// 重启整个客户端
func (s *Server) RestartClient(clientID string) error
```

### 3.4 REST API (`internal/server/router/api.go`)

```go
// 客户端控制
POST /api/client/{id}/restart           // 重启整个客户端

// 插件控制
POST /api/client/{id}/plugin/{name}/stop      // 停止插件
POST /api/client/{id}/plugin/{name}/restart   // 重启插件
PUT  /api/client/{id}/plugin/{name}/config    // 更新配置并可选重启
```

### 3.5 前端界面 (`web/src/views/ClientView.vue`)

在客户端详情页添加控制按钮：
- **客户端级别**: "重启客户端" 按钮
- **插件级别**: 每个运行中的插件显示 "停止"、"重启"、"配置" 按钮

---

## 实施顺序

### 阶段 1: 清理 WASM 代码
1. 删除 WASM 相关文件和代码
2. 更新数据库 schema
3. 清理依赖

### 阶段 2: 协议动态配置
1. 定义 ConfigField 和 RuleSchema 类型
2. 实现内置协议的配置模式
3. 添加 `/api/rule-schemas` 端点
4. 更新前端规则编辑界面

### 阶段 3: JS 插件管理优化
1. 扩展 JSPlugin 结构和数据库
2. 实现插件配置 API
3. 重新设计前端插件管理界面

### 阶段 4: 动态重启功能
1. 实现客户端 pluginStop 处理
2. 实现服务端重启方法
3. 添加 REST API 端点
4. 更新前端添加控制按钮

### 阶段 5: 测试和文档
1. 端到端测试
2. 更新 CLAUDE.md

---

## 文件变更清单

### 后端 Go 文件
- `pkg/plugin/types.go` - 添加 ConfigField, RuleSchema
- `pkg/plugin/schema.go` (新建) - 内置协议配置模式
- `pkg/protocol/message.go` - 新增消息类型
- `internal/server/db/interface.go` - 更新接口
- `internal/server/db/sqlite.go` - 更新数据库操作
- `internal/server/router/api.go` - 新增 API 端点
- `internal/server/tunnel/server.go` - 重启控制方法
- `internal/client/tunnel/client.go` - 插件停止/重启处理

### 前端文件
- `web/src/types/index.ts` - 类型定义
- `web/src/api/index.ts` - API 调用
- `web/src/views/ClientView.vue` - 规则配置和重启控制
- `web/src/views/PluginsView.vue` - 插件管理界面
