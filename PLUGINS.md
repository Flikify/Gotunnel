# GoTunnel 插件开发指南

本文档介绍如何为 GoTunnel 开发 JS 插件。JS 插件基于 [goja](https://github.com/dop251/goja) 运行时，运行在客户端上。

## 目录

- [快速开始](#快速开始)
- [插件结构](#插件结构)
- [API 参考](#api-参考)
- [示例插件](#示例插件)
- [插件签名](#插件签名)
- [发布到商店](#发布到商店)

---

## 快速开始

### 最小插件示例

```javascript
// 必须：定义插件元数据
function metadata() {
    return {
        name: "my-plugin",
        version: "1.0.0",
        type: "app",
        description: "My first plugin",
        author: "Your Name"
    };
}

// 可选：插件启动时调用
function start() {
    log("Plugin started");
}

// 必须：处理连接
function handleConn(conn) {
    // 处理连接逻辑
    conn.Close();
}

// 可选：插件停止时调用
function stop() {
    log("Plugin stopped");
}
```

---

## 插件结构

### 生命周期函数

| 函数 | 必须 | 说明 |
|------|------|------|
| `metadata()` | 否 | 返回插件元数据，不定义则使用默认值 |
| `start()` | 否 | 插件启动时调用 |
| `handleConn(conn)` | 是 | 处理每个连接 |
| `stop()` | 否 | 插件停止时调用 |

### 元数据字段

```javascript
function metadata() {
    return {
        name: "plugin-name",      // 插件名称
        version: "1.0.0",         // 版本号
        type: "app",              // 类型: "app" (应用插件)
        description: "描述",      // 插件描述
        author: "作者"            // 作者名称
    };
}
```

---

## API 参考

### 基础 API

#### `log(message)`

输出日志信息。

```javascript
log("Hello, World!");
// 输出: [JS:plugin-name] Hello, World!
```

#### `config(key)`

获取插件配置值。

```javascript
var port = config("port");
var host = config("host") || "127.0.0.1";
```

---

### 连接 API (conn)

`handleConn` 函数接收的 `conn` 对象提供以下方法：

#### `conn.Read(size)`

读取数据，返回字节数组，失败返回 `null`。

```javascript
var data = conn.Read(1024);
if (data) {
    log("Received " + data.length + " bytes");
}
```

#### `conn.Write(data)`

写入数据，返回写入的字节数。

```javascript
var written = conn.Write(data);
log("Wrote " + written + " bytes");
```

#### `conn.Close()`

关闭连接。

```javascript
conn.Close();
```

---

### 文件系统 API (fs)

所有文件操作都在沙箱中执行，有路径和大小限制。

#### `fs.readFile(path)`

读取文件内容。

```javascript
var result = fs.readFile("/path/to/file.txt");
if (result.error) {
    log("Error: " + result.error);
} else {
    log("Content: " + result.data);
}
```

#### `fs.writeFile(path, content)`

写入文件内容。

```javascript
var result = fs.writeFile("/path/to/file.txt", "Hello");
if (result.ok) {
    log("File written");
} else {
    log("Error: " + result.error);
}
```

#### `fs.readDir(path)`

读取目录内容。

```javascript
var result = fs.readDir("/path/to/dir");
if (!result.error) {
    for (var i = 0; i < result.entries.length; i++) {
        var entry = result.entries[i];
        log(entry.name + " - " + (entry.isDir ? "DIR" : entry.size + " bytes"));
    }
}
```

#### `fs.stat(path)`

获取文件信息。

```javascript
var result = fs.stat("/path/to/file");
if (!result.error) {
    log("Name: " + result.name);
    log("Size: " + result.size);
    log("IsDir: " + result.isDir);
    log("ModTime: " + result.modTime);
}
```

#### `fs.exists(path)`

检查文件是否存在。

```javascript
var result = fs.exists("/path/to/file");
if (result.exists) {
    log("File exists");
}
```

#### `fs.mkdir(path)`

创建目录。

```javascript
var result = fs.mkdir("/path/to/new/dir");
if (result.ok) {
    log("Directory created");
}
```

#### `fs.remove(path)`

删除文件或目录。

```javascript
var result = fs.remove("/path/to/file");
if (result.ok) {
    log("Removed");
}
```

---

### HTTP API (http)

用于构建简单的 HTTP 服务。

#### `http.serve(conn, handler)`

处理 HTTP 请求。

```javascript
function handleConn(conn) {
    http.serve(conn, function(req) {
        return {
            status: 200,
            contentType: "application/json",
            body: http.json({ message: "Hello", path: req.path })
        };
    });
}
```

**请求对象 (req):**

| 字段 | 类型 | 说明 |
|------|------|------|
| `method` | string | HTTP 方法 (GET, POST, etc.) |
| `path` | string | 请求路径 |
| `body` | string | 请求体 |

**响应对象:**

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | number | HTTP 状态码 (默认 200) |
| `contentType` | string | Content-Type (默认 application/json) |
| `body` | string | 响应体 |

#### `http.json(data)`

将对象序列化为 JSON 字符串。

```javascript
var jsonStr = http.json({ name: "test", value: 123 });
// 返回: '{"name":"test","value":123}'
```

#### `http.sendFile(conn, filePath)`

发送文件作为 HTTP 响应。

```javascript
function handleConn(conn) {
    http.sendFile(conn, "/path/to/index.html");
}
```

---

## 示例插件

### Echo 服务

```javascript
function metadata() {
    return {
        name: "echo",
        version: "1.0.0",
        type: "app",
        description: "Echo back received data"
    };
}

function handleConn(conn) {
    while (true) {
        var data = conn.Read(4096);
        if (!data || data.length === 0) {
            break;
        }
        conn.Write(data);
    }
    conn.Close();
}
```

### HTTP 文件服务器

```javascript
function metadata() {
    return {
        name: "file-server",
        version: "1.0.0",
        type: "app",
        description: "Simple HTTP file server"
    };
}

var rootDir = "";

function start() {
    rootDir = config("root") || "/tmp";
    log("Serving files from: " + rootDir);
}

function handleConn(conn) {
    http.serve(conn, function(req) {
        if (req.method === "GET") {
            var filePath = rootDir + req.path;
            if (req.path === "/") {
                filePath = rootDir + "/index.html";
            }

            var stat = fs.stat(filePath);
            if (stat.error) {
                return { status: 404, body: "Not Found" };
            }

            if (stat.isDir) {
                return listDirectory(filePath);
            }

            var file = fs.readFile(filePath);
            if (file.error) {
                return { status: 500, body: file.error };
            }

            return {
                status: 200,
                contentType: "text/html",
                body: file.data
            };
        }
        return { status: 405, body: "Method Not Allowed" };
    });
}

function listDirectory(path) {
    var result = fs.readDir(path);
    if (result.error) {
        return { status: 500, body: result.error };
    }

    var html = "<html><body><h1>Directory Listing</h1><ul>";
    for (var i = 0; i < result.entries.length; i++) {
        var e = result.entries[i];
        html += "<li><a href='" + e.name + "'>" + e.name + "</a></li>";
    }
    html += "</ul></body></html>";

    return { status: 200, contentType: "text/html", body: html };
}
```

### JSON API 服务

```javascript
function metadata() {
    return {
        name: "api-server",
        version: "1.0.0",
        type: "app",
        description: "JSON API server"
    };
}

var counter = 0;

function handleConn(conn) {
    http.serve(conn, function(req) {
        if (req.path === "/api/status") {
            return {
                status: 200,
                body: http.json({
                    status: "ok",
                    counter: counter++,
                    timestamp: Date.now()
                })
            };
        }

        if (req.path === "/api/echo" && req.method === "POST") {
            return {
                status: 200,
                body: http.json({
                    received: req.body
                })
            };
        }

        return {
            status: 404,
            body: http.json({ error: "Not Found" })
        };
    });
}
```

---

## 插件签名

为了安全，JS 插件需要官方签名才能运行。

### 签名格式

签名文件 (`.sig`) 包含 Base64 编码的签名数据：

```json
{
  "payload": {
    "name": "plugin-name",
    "version": "1.0.0",
    "checksum": "sha256-hash",
    "key_id": "official-v1"
  },
  "signature": "base64-signature"
}
```

### 获取签名

1. 提交插件到官方仓库
2. 通过审核后获得签名
3. 将 `.js` 和 `.sig` 文件一起分发

---

## 发布到商店

### 商店 JSON 格式

插件商店使用 `store.json` 文件索引所有插件：

```json
[
  {
    "name": "echo",
    "version": "1.0.0",
    "type": "app",
    "description": "Echo service plugin",
    "author": "GoTunnel",
    "icon": "https://example.com/icon.png",
    "download_url": "https://example.com/plugins/echo.js"
  }
]
```

### 提交流程

1. Fork 官方插件仓库
2. 添加插件文件到 `plugins/` 目录
3. 更新 `store.json`
4. 提交 Pull Request
5. 等待审核和签名

---

## 沙箱限制

为了安全，JS 插件运行在沙箱环境中：

| 限制项 | 默认值 |
|--------|--------|
| 最大读取文件大小 | 10 MB |
| 最大写入文件大小 | 10 MB |
| 允许读取路径 | 插件数据目录 |
| 允许写入路径 | 插件数据目录 |

---

## 调试技巧

### 日志输出

使用 `log()` 函数输出调试信息：

```javascript
log("Debug: variable = " + JSON.stringify(variable));
```

### 错误处理

始终检查 API 返回的错误：

```javascript
var result = fs.readFile(path);
if (result.error) {
    log("Error reading file: " + result.error);
    return;
}
```

### 配置测试

在 Web 控制台的插件管理页面安装并配置插件，或通过 API 安装：

```bash
# 安装 JS 插件到客户端
POST /api/client/{id}/plugin/js/install
Content-Type: application/json
{
  "plugin_name": "my-plugin",
  "source": "function metadata() {...}",
  "rule_name": "my-rule",
  "remote_port": 8080,
  "config": {"debug": "true"},
  "auto_start": true
}
```

---

## 常见问题

**Q: 插件无法加载？**

A: 检查签名文件是否存在且有效。

**Q: 文件操作失败？**

A: 确认路径在沙箱允许范围内。

**Q: 如何获取客户端 IP？**

A: 目前 API 不支持，计划在后续版本添加。

---

## 更新日志

### v1.0.0

- 初始版本
- 支持基础 API: log, config
- 支持连接 API: Read, Write, Close
- 支持文件系统 API: fs.*
- 支持 HTTP API: http.*
