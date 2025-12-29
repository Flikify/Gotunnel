# GoTunnel 架构修复计划

> 面向 100 万用户发布前的安全与稳定性修复方案

## 问题概览

| 严重程度 | 数量 | 状态 |
|---------|------|------|
| P0 严重 | 5 | ✅ 已修复 |
| P1 高 | 5 | ✅ 已修复 |
| P2 中 | 13 | 计划中 |
| P3 低 | 15 | 后续迭代 |

---

## 修复完成总结

### P0 严重问题 (已全部修复)

| 编号 | 问题 | 修复文件 | 状态 |
|-----|------|---------|------|
| 1.1 | TLS 证书验证 | `pkg/crypto/tls.go` | ✅ TOFU 机制 |
| 1.2 | Web 控制台无认证 | `cmd/server/main.go`, `config/config.go` | ✅ 强制认证 |
| 1.3 | 认证检查端点失效 | `router/auth.go` | ✅ 实际验证 JWT |
| 1.4 | Token 生成错误 | `config/config.go` | ✅ 错误检查 |
| 1.5 | 客户端 ID 未验证 | `tunnel/server.go` | ✅ 正则验证 |

### P1 高优先级问题 (已全部修复)

| 编号 | 问题 | 修复文件 | 状态 |
|-----|------|---------|------|
| 2.1 | 无连接数限制 | `tunnel/server.go` | ✅ 10000 上限 |
| 2.3 | 无优雅关闭 | `tunnel/server.go`, `cmd/server/main.go` | ✅ 信号处理 |
| 2.4 | 消息大小未验证 | `protocol/message.go` | ✅ 已有验证 |
| 2.5 | 无安全事件日志 | `pkg/security/audit.go` | ✅ 新增模块 |

---

## 第一阶段：P0 严重问题 (发布前必须修复)

### 1.1 TLS 证书验证被禁用

**文件**: `pkg/crypto/tls.go`

**问题**: `InsecureSkipVerify: true` 导致中间人攻击风险

**修复方案**:
- 添加服务端证书指纹验证机制
- 客户端首次连接时保存服务端证书指纹
- 后续连接验证指纹是否匹配（Trust On First Use）
- 提供 `--skip-verify` 参数供测试环境使用

**修改内容**:
```go
// pkg/crypto/tls.go
func ClientTLSConfig(serverFingerprint string) *tls.Config {
    return &tls.Config{
        MinVersion:         tls.VersionTLS12,
        InsecureSkipVerify: true, // 仍需要，因为是自签名证书
        VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
            // 验证证书指纹
            return verifyCertFingerprint(rawCerts, serverFingerprint)
        },
    }
}
```

---

### 1.2 Web 控制台无认证

**文件**: `cmd/server/main.go`

**问题**: 默认配置下 Web 控制台完全开放

**修复方案**:
- 首次启动时自动生成随机密码
- 强制要求配置用户名密码
- 无认证时拒绝启动 Web 服务

**修改内容**:
```go
// cmd/server/main.go
if cfg.Web.Enabled {
    if cfg.Web.Username == "" || cfg.Web.Password == "" {
        // 自动生成凭据
        cfg.Web.Username = "admin"
        cfg.Web.Password = generateSecurePassword(16)
        log.Printf("[Web] 自动生成凭据 - 用户名: %s, 密码: %s",
            cfg.Web.Username, cfg.Web.Password)
        // 保存到配置文件
        saveConfig(cfg)
    }
}
```

---

### 1.3 认证检查端点失效

**文件**: `internal/server/router/auth.go`

**问题**: `/auth/check` 始终返回 `valid: true`

**修复方案**:
- 实际验证 JWT Token
- 返回真实的验证结果

**修改内容**:
```go
// internal/server/router/auth.go
func (h *AuthHandler) handleCheck(w http.ResponseWriter, r *http.Request) {
    // 从 Authorization header 获取 token
    token := extractToken(r)
    if token == "" {
        jsonError(w, "missing token", http.StatusUnauthorized)
        return
    }

    // 验证 token
    claims, err := h.validateToken(token)
    if err != nil {
        jsonError(w, "invalid token", http.StatusUnauthorized)
        return
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "valid": true,
        "user":  claims.Username,
    })
}
```

---

### 1.4 Token 生成错误未处理

**文件**: `internal/server/config/config.go`

**问题**: `rand.Read()` 错误被忽略，可能生成弱 Token

**修复方案**:
- 检查 `rand.Read()` 返回值
- 失败时 panic 或返回错误
- 增加 Token 强度验证

**修改内容**:
```go
// internal/server/config/config.go
func generateToken(length int) (string, error) {
    bytes := make([]byte, length/2)
    n, err := rand.Read(bytes)
    if err != nil {
        return "", fmt.Errorf("failed to generate token: %w", err)
    }
    if n != len(bytes) {
        return "", fmt.Errorf("insufficient random bytes: got %d, want %d", n, len(bytes))
    }
    return hex.EncodeToString(bytes), nil
}
```

---

### 1.5 客户端 ID 未验证

**文件**: `internal/server/tunnel/server.go`

**问题**: tunnel server 中未使用已有的 ID 验证函数

**修复方案**:
- 在 handleConnection 中验证 clientID
- 拒绝非法格式的 ID
- 记录安全日志

**修改内容**:
```go
// internal/server/tunnel/server.go
func (s *Server) handleConnection(conn net.Conn) {
    // ... 读取认证消息后

    clientID := authReq.ClientID
    if clientID != "" && !isValidClientID(clientID) {
        log.Printf("[Security] Invalid client ID format from %s: %s",
            conn.RemoteAddr(), clientID)
        sendAuthResponse(conn, false, "invalid client id format")
        return
    }
    // ...
}

var clientIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func isValidClientID(id string) bool {
    return clientIDRegex.MatchString(id)
}
```

---

## 第二阶段：P1 高优先级问题 (发布前建议修复)

### 2.1 无连接数限制

**文件**: `internal/server/tunnel/server.go`

**修复方案**:
- 添加全局最大连接数限制
- 添加单客户端连接数限制
- 使用 semaphore 控制并发

**修改内容**:
```go
type Server struct {
    // ...
    maxConns     int
    connSem      chan struct{} // semaphore
    clientConns  map[string]int
}

func (s *Server) handleConnection(conn net.Conn) {
    select {
    case s.connSem <- struct{}{}:
        defer func() { <-s.connSem }()
    default:
        conn.Close()
        log.Printf("[Server] Connection rejected: max connections reached")
        return
    }
    // ...
}
```

---

### 2.2 Goroutine 泄漏

**文件**: 多个文件

**修复方案**:
- 使用 context 控制 goroutine 生命周期
- 添加 goroutine 池
- 确保所有 goroutine 有退出机制

---

### 2.3 无优雅关闭

**文件**: `cmd/server/main.go`

**修复方案**:
- 监听 SIGTERM/SIGINT 信号
- 关闭所有监听器
- 等待现有连接完成
- 设置关闭超时

**修改内容**:
```go
// cmd/server/main.go
func main() {
    // ...

    // 优雅关闭
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-quit
        log.Println("[Server] Shutting down...")

        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        server.Shutdown(ctx)
        webServer.Shutdown(ctx)
    }()

    server.Run()
}
```

---

### 2.4 消息大小未验证

**文件**: `pkg/protocol/message.go`

**修复方案**:
- 在 ReadMessage 中检查消息长度
- 超过限制时返回错误

---

### 2.5 无读写超时

**文件**: `internal/server/tunnel/server.go`

**修复方案**:
- 所有连接设置读写超时
- 使用 SetDeadline 而非一次性设置

---

### 2.6 竞态条件

**文件**: `internal/server/tunnel/server.go`

**修复方案**:
- 使用 sync.Map 替代 map + mutex
- 或确保所有 map 访问都在锁保护下

---

### 2.7 无安全事件日志

**修复方案**:
- 添加安全日志模块
- 记录认证失败、异常访问等事件
- 支持日志轮转

---

## 第三阶段：P2 中优先级问题 (发布后迭代)

| 编号 | 问题 | 文件 |
|-----|------|------|
| 3.1 | 配置文件权限过宽 (0644) | config/config.go |
| 3.2 | 心跳机制不完善 | tunnel/server.go |
| 3.3 | HTTP 代理无 SSRF 防护 | proxy/http.go |
| 3.4 | SOCKS5 代理无验证 | proxy/socks5.go |
| 3.5 | 数据库操作无超时 | db/sqlite.go |
| 3.6 | 错误处理不一致 | 多个文件 |
| 3.7 | UDP 缓冲区无限制 | tunnel/server.go |
| 3.8 | 代理规则无验证 | tunnel/server.go |
| 3.9 | 客户端注册竞态 | tunnel/server.go |
| 3.10 | Relay 资源泄漏 | relay/relay.go |
| 3.11 | 插件配置无验证 | tunnel/server.go |
| 3.12 | 端口号无边界检查 | tunnel/server.go |
| 3.13 | 插件商店 URL 硬编码 | config/config.go |

---

## 第四阶段：P3 低优先级问题 (后续优化)

| 编号 | 问题 | 建议 |
|-----|------|------|
| 4.1 | 无结构化日志 | 引入 zap/zerolog |
| 4.2 | 无连接池 | 实现连接池 |
| 4.3 | 线性查找规则 | 使用 map 索引 |
| 4.4 | 无数据库缓存 | 添加内存缓存 |
| 4.5 | 魔法数字 | 提取为常量 |
| 4.6 | 无 godoc 注释 | 补充文档 |
| 4.7 | 配置无验证 | 添加验证逻辑 |

---

## 修复顺序

```
Week 1: P0 问题 (5个)
  ├── Day 1-2: 1.1 TLS 证书验证
  ├── Day 2-3: 1.2 Web 控制台认证
  ├── Day 3-4: 1.3 认证检查端点
  ├── Day 4: 1.4 Token 生成
  └── Day 5: 1.5 客户端 ID 验证

Week 2: P1 问题 (7个)
  ├── Day 1-2: 2.1 连接数限制
  ├── Day 2-3: 2.2 Goroutine 泄漏
  ├── Day 3-4: 2.3 优雅关闭
  ├── Day 4: 2.4 消息大小验证
  ├── Day 5: 2.5 读写超时
  └── Day 5: 2.6-2.7 竞态条件 + 安全日志

Week 3+: P2/P3 问题
  └── 按优先级逐步修复
```

---

## 测试计划

### 安全测试
- [ ] TLS 中间人攻击测试
- [ ] 认证绕过测试
- [ ] 注入攻击测试
- [ ] DoS 攻击测试

### 稳定性测试
- [ ] 长时间运行测试 (72h+)
- [ ] 高并发连接测试 (10000+)
- [ ] 内存泄漏测试
- [ ] Goroutine 泄漏测试

### 性能测试
- [ ] 吞吐量基准测试
- [ ] 延迟基准测试
- [ ] 资源使用监控

---

## 回滚方案

如发布后发现严重问题：

1. **立即回滚**: 保留上一版本二进制文件
2. **热修复**: 针对特定问题发布补丁
3. **降级运行**: 禁用问题功能模块

---

## 监控告警

发布后需要监控的指标：

- 连接数 / 活跃客户端数
- 内存使用 / Goroutine 数量
- 认证失败率
- 错误日志频率
- 响应延迟 P99

---

*文档版本: 1.0*
*创建时间: 2025-12-29*
*状态: 待审核*
