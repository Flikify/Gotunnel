package script

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dop251/goja"
	"github.com/gotunnel/pkg/plugin"
)

// JSPlugin JavaScript 脚本插件
type JSPlugin struct {
	name     string
	source   string
	vm       *goja.Runtime
	metadata plugin.Metadata
	config         map[string]string
	sandbox        *Sandbox
	running        bool
	mu             sync.Mutex
	eventListeners map[string][]func(goja.Value)
	storagePath    string
	apiHandlers    map[string]map[string]goja.Callable // method -> path -> handler
}

// NewJSPlugin 从 JS 源码创建插件
func NewJSPlugin(name, source string) (*JSPlugin, error) {
	p := &JSPlugin{
		name:           name,
		source:         source,
		vm:             goja.New(),
		sandbox:        DefaultSandbox(),
		eventListeners: make(map[string][]func(goja.Value)),
		storagePath:    filepath.Join("plugin_data", name+".json"),
		apiHandlers:    make(map[string]map[string]goja.Callable),
	}

	// 确保存储目录存在
	os.MkdirAll("plugin_data", 0755)

	if err := p.init(); err != nil {
		return nil, err
	}

	return p, nil
}

// SetSandbox 设置沙箱配置
func (p *JSPlugin) SetSandbox(sandbox *Sandbox) {
	p.sandbox = sandbox
}

// init 初始化 JS 运行时
func (p *JSPlugin) init() error {
	// 设置栈深度限制（防止递归攻击）
	if p.sandbox.MaxStackDepth > 0 {
		p.vm.SetMaxCallStackSize(p.sandbox.MaxStackDepth)
	}

	// 注入基础 API
	p.vm.Set("log", p.jsLog)
	
	// Config API (兼容旧的 config() 调用，同时支持 config.get/getAll)
	p.vm.Set("config", p.jsGetConfig)
	if configObj := p.vm.Get("config"); configObj != nil {
		obj := configObj.ToObject(p.vm)
		obj.Set("get", p.jsGetConfig)
		obj.Set("getAll", p.jsGetAllConfig)
	}

	// 注入增强 API
	p.vm.Set("logger", p.createLoggerAPI())
	p.vm.Set("storage", p.createStorageAPI())
	p.vm.Set("event", p.createEventAPI())
	p.vm.Set("request", p.createRequestAPI())
	p.vm.Set("notify", p.createNotifyAPI())

	// 注入文件 API
	p.vm.Set("fs", p.createFsAPI())

	// 注入 HTTP API
	p.vm.Set("http", p.createHttpAPI())

	// 注入路由 API
	p.vm.Set("api", p.createRouteAPI())

	// 执行脚本
	_, err := p.vm.RunString(p.source)
	if err != nil {
		return fmt.Errorf("run script: %w", err)
	}

	// 获取元数据
	if err := p.loadMetadata(); err != nil {
		return err
	}

	return nil
}

// loadMetadata 从 JS 获取元数据
func (p *JSPlugin) loadMetadata() error {
	fn, ok := goja.AssertFunction(p.vm.Get("metadata"))
	if !ok {
		// 使用默认元数据
		p.metadata = plugin.Metadata{
			Name:   p.name,
			Type:   plugin.PluginTypeApp,
			Source: plugin.PluginSourceScript,
			RunAt:  plugin.SideClient,
		}
		return nil
	}

	result, err := fn(goja.Undefined())
	if err != nil {
		return err
	}

	obj := result.ToObject(p.vm)
	p.metadata = plugin.Metadata{
		Name:        getString(obj, "name", p.name),
		Version:     getString(obj, "version", "1.0.0"),
		Type:        plugin.PluginType(getString(obj, "type", "app")),
		Source:      plugin.PluginSourceScript,
		RunAt:       plugin.Side(getString(obj, "run_at", "client")),
		Description: getString(obj, "description", ""),
		Author:      getString(obj, "author", ""),
	}
	return nil
}

// Metadata 返回插件元数据
func (p *JSPlugin) Metadata() plugin.Metadata {
	return p.metadata
}

// Init 初始化插件配置
func (p *JSPlugin) Init(config map[string]string) error {
	p.config = config
	// p.vm.Set("config", config) // Do not overwrite the config API
	return nil
}

// Start 启动插件
func (p *JSPlugin) Start() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return "", nil
	}

	fn, ok := goja.AssertFunction(p.vm.Get("start"))
	if ok {
		_, err := fn(goja.Undefined())
		if err != nil {
			return "", err
		}
	}

	p.running = true
	return "script-plugin", nil
}

// HandleConn 处理连接
func (p *JSPlugin) HandleConn(conn net.Conn) error {
	defer conn.Close()

	// 创建连接包装器
	jsConn := newJSConn(conn)
	p.vm.Set("conn", jsConn)

	fn, ok := goja.AssertFunction(p.vm.Get("handleConn"))
	if !ok {
		return fmt.Errorf("handleConn not defined")
	}

	_, err := fn(goja.Undefined(), p.vm.ToValue(jsConn))
	return err
}

// Stop 停止插件
func (p *JSPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	fn, ok := goja.AssertFunction(p.vm.Get("stop"))
	if ok {
		fn(goja.Undefined())
	}

	p.running = false
	return nil
}

// jsLog JS 日志函数
func (p *JSPlugin) jsLog(msg string) {
	fmt.Printf("[JS:%s] %s\n", p.name, msg)
}

// jsGetConfig 获取配置
func (p *JSPlugin) jsGetConfig(key string) string {
	if p.config == nil {
		return ""
	}
	return p.config[key]
}

// getString 从 JS 对象获取字符串
func getString(obj *goja.Object, key, def string) string {
	v := obj.Get(key)
	if v == nil || goja.IsUndefined(v) {
		return def
	}
	return v.String()
}

// jsConn JS 连接包装器
type jsConn struct {
	conn net.Conn
}

func newJSConn(conn net.Conn) *jsConn {
	return &jsConn{conn: conn}
}

func (c *jsConn) Read(size int) []byte {
	buf := make([]byte, size)
	n, err := c.conn.Read(buf)
	if err != nil {
		return nil
	}
	return buf[:n]
}

func (c *jsConn) Write(data []byte) int {
	n, _ := c.conn.Write(data)
	return n
}

func (c *jsConn) Close() {
	c.conn.Close()
}

// =============================================================================
// 文件系统 API
// =============================================================================

// createFsAPI 创建文件系统 API
func (p *JSPlugin) createFsAPI() map[string]interface{} {
	return map[string]interface{}{
		"readFile":  p.fsReadFile,
		"writeFile": p.fsWriteFile,
		"readDir":   p.fsReadDir,
		"stat":      p.fsStat,
		"exists":    p.fsExists,
		"mkdir":     p.fsMkdir,
		"remove":    p.fsRemove,
	}
}

func (p *JSPlugin) fsReadFile(path string) map[string]interface{} {
	if err := p.sandbox.ValidateReadPath(path); err != nil {
		return map[string]interface{}{"error": err.Error(), "data": ""}
	}

	info, err := os.Stat(path)
	if err != nil {
		return map[string]interface{}{"error": err.Error(), "data": ""}
	}
	if info.Size() > p.sandbox.MaxReadSize {
		return map[string]interface{}{"error": "file too large", "data": ""}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]interface{}{"error": err.Error(), "data": ""}
	}
	return map[string]interface{}{"error": "", "data": string(data)}
}

func (p *JSPlugin) fsWriteFile(path, content string) map[string]interface{} {
	if err := p.sandbox.ValidateWritePath(path); err != nil {
		return map[string]interface{}{"error": err.Error(), "ok": false}
	}

	if int64(len(content)) > p.sandbox.MaxWriteSize {
		return map[string]interface{}{"error": "content too large", "ok": false}
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return map[string]interface{}{"error": err.Error(), "ok": false}
	}
	return map[string]interface{}{"error": "", "ok": true}
}

func (p *JSPlugin) fsReadDir(path string) map[string]interface{} {
	if err := p.sandbox.ValidateReadPath(path); err != nil {
		return map[string]interface{}{"error": err.Error(), "entries": nil}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return map[string]interface{}{"error": err.Error(), "entries": nil}
	}
	var result []map[string]interface{}
	for _, e := range entries {
		info, _ := e.Info()
		result = append(result, map[string]interface{}{
			"name":  e.Name(),
			"isDir": e.IsDir(),
			"size":  info.Size(),
		})
	}
	return map[string]interface{}{"error": "", "entries": result}
}

func (p *JSPlugin) fsStat(path string) map[string]interface{} {
	if err := p.sandbox.ValidateReadPath(path); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	info, err := os.Stat(path)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{
		"error":   "",
		"name":    info.Name(),
		"size":    info.Size(),
		"isDir":   info.IsDir(),
		"modTime": info.ModTime().Unix(),
	}
}

func (p *JSPlugin) fsExists(path string) map[string]interface{} {
	if err := p.sandbox.ValidateReadPath(path); err != nil {
		return map[string]interface{}{"error": err.Error(), "exists": false}
	}
	_, err := os.Stat(path)
	return map[string]interface{}{"error": "", "exists": err == nil}
}

func (p *JSPlugin) fsMkdir(path string) map[string]interface{} {
	if err := p.sandbox.ValidateWritePath(path); err != nil {
		return map[string]interface{}{"error": err.Error(), "ok": false}
	}
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return map[string]interface{}{"error": err.Error(), "ok": false}
	}
	return map[string]interface{}{"error": "", "ok": true}
}

func (p *JSPlugin) fsRemove(path string) map[string]interface{} {
	if err := p.sandbox.ValidateWritePath(path); err != nil {
		return map[string]interface{}{"error": err.Error(), "ok": false}
	}
	err := os.RemoveAll(path)
	if err != nil {
		return map[string]interface{}{"error": err.Error(), "ok": false}
	}
	return map[string]interface{}{"error": "", "ok": true}
}

// =============================================================================
// HTTP 服务 API
// =============================================================================

// createHttpAPI 创建 HTTP API
func (p *JSPlugin) createHttpAPI() map[string]interface{} {
	return map[string]interface{}{
		"serve":    p.httpServe,
		"json":     p.httpJSON,
		"sendFile": p.httpSendFile,
	}
}

// httpServe 启动 HTTP 服务处理连接
func (p *JSPlugin) httpServe(conn net.Conn, handler func(map[string]interface{}) map[string]interface{}) {
	defer conn.Close()

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	req := parseHTTPRequest(buf[:n])
	resp := handler(req)
	writeHTTPResponse(conn, resp)
}

func (p *JSPlugin) httpJSON(data interface{}) string {
	b, _ := json.Marshal(data)
	return string(b)
}

func (p *JSPlugin) httpSendFile(conn net.Conn, filePath string) {
	f, err := os.Open(filePath)
	if err != nil {
		conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
		return
	}
	defer f.Close()

	info, _ := f.Stat()
	contentType := getContentType(filePath)

	header := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: %s\r\nContent-Length: %d\r\n\r\n",
		contentType, info.Size())
	conn.Write([]byte(header))
	io.Copy(conn, f)
}

// parseHTTPRequest 解析 HTTP 请求
func parseHTTPRequest(data []byte) map[string]interface{} {
	lines := string(data)
	req := map[string]interface{}{
		"method": "GET",
		"path":   "/",
		"body":   "",
	}

	// 解析请求行
	if idx := indexOf(lines, " "); idx > 0 {
		req["method"] = lines[:idx]
		rest := lines[idx+1:]
		if idx2 := indexOf(rest, " "); idx2 > 0 {
			req["path"] = rest[:idx2]
		}
	}

	// 解析 body
	if idx := indexOf(lines, "\r\n\r\n"); idx > 0 {
		req["body"] = lines[idx+4:]
	}

	return req
}

// writeHTTPResponse 写入 HTTP 响应
func writeHTTPResponse(conn net.Conn, resp map[string]interface{}) {
	status := 200
	if s, ok := resp["status"].(int); ok {
		status = s
	}

	body := ""
	if b, ok := resp["body"].(string); ok {
		body = b
	}

	contentType := "application/json"
	if ct, ok := resp["contentType"].(string); ok {
		contentType = ct
	}

	header := fmt.Sprintf("HTTP/1.1 %d OK\r\nContent-Type: %s\r\nContent-Length: %d\r\n\r\n",
		status, contentType, len(body))
	conn.Write([]byte(header + body))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func getContentType(path string) string {
	ext := filepath.Ext(path)
	types := map[string]string{
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".gif":  "image/gif",
		".txt":  "text/plain",
	}
	if ct, ok := types[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}

// =============================================================================
// Logger API
// =============================================================================

func (p *JSPlugin) createLoggerAPI() map[string]interface{} {
	return map[string]interface{}{
		"info":  func(msg string) { fmt.Printf("[JS:%s][INFO] %s\n", p.name, msg) },
		"warn":  func(msg string) { fmt.Printf("[JS:%s][WARN] %s\n", p.name, msg) },
		"error": func(msg string) { fmt.Printf("[JS:%s][ERROR] %s\n", p.name, msg) },
	}
}

// =============================================================================
// Config API Enhancements
// =============================================================================

func (p *JSPlugin) jsGetAllConfig() map[string]string {
	if p.config == nil {
		return map[string]string{}
	}
	return p.config
}

// =============================================================================
// Storage API
// =============================================================================

func (p *JSPlugin) createStorageAPI() map[string]interface{} {
	return map[string]interface{}{
		"get":    p.storageGet,
		"set":    p.storageSet,
		"delete": p.storageDelete,
		"keys":   p.storageKeys,
	}
}

func (p *JSPlugin) loadStorage() map[string]interface{} {
	data := make(map[string]interface{})
	if _, err := os.Stat(p.storagePath); err == nil {
		content, _ := os.ReadFile(p.storagePath)
		json.Unmarshal(content, &data)
	}
	return data
}

func (p *JSPlugin) saveStorage(data map[string]interface{}) {
	content, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile(p.storagePath, content, 0644)
}

func (p *JSPlugin) storageGet(key string, def interface{}) interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	data := p.loadStorage()
	if v, ok := data[key]; ok {
		return v
	}
	return def
}

func (p *JSPlugin) storageSet(key string, value interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	data := p.loadStorage()
	data[key] = value
	p.saveStorage(data)
}

func (p *JSPlugin) storageDelete(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	data := p.loadStorage()
	delete(data, key)
	p.saveStorage(data)
}

func (p *JSPlugin) storageKeys() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	data := p.loadStorage()
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	return keys
}

// =============================================================================
// Event API
// =============================================================================

func (p *JSPlugin) createEventAPI() map[string]interface{} {
	return map[string]interface{}{
		"on":   p.eventOn,
		"emit": p.eventEmit,
		"off":  p.eventOff,
	}
}

func (p *JSPlugin) eventOn(event string, callback func(goja.Value)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.eventListeners[event] = append(p.eventListeners[event], callback)
}

func (p *JSPlugin) eventEmit(event string, data interface{}) {
	p.mu.Lock()
	listeners := p.eventListeners[event]
	p.mu.Unlock() // 释放锁以允许回调中操作

	val := p.vm.ToValue(data)
	for _, cb := range listeners {
		cb(val)
	}
}

func (p *JSPlugin) eventOff(event string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.eventListeners, event)
}

// =============================================================================
// Request API (HTTP Client)
// =============================================================================

func (p *JSPlugin) createRequestAPI() map[string]interface{} {
	return map[string]interface{}{
		"get":  p.requestGet,
		"post": p.requestPost,
	}
}

func (p *JSPlugin) requestGet(url string) map[string]interface{} {
	resp, err := http.Get(url)
	if err != nil {
		return map[string]interface{}{"error": err.Error(), "status": 0}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return map[string]interface{}{
		"status": resp.StatusCode,
		"body":   string(body),
		"error":  "",
	}
}

func (p *JSPlugin) requestPost(url string, contentType, data string) map[string]interface{} {
	resp, err := http.Post(url, contentType, strings.NewReader(data))
	if err != nil {
		return map[string]interface{}{"error": err.Error(), "status": 0}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return map[string]interface{}{
		"status": resp.StatusCode,
		"body":   string(body),
		"error":  "",
	}
}

// =============================================================================
// Notify API
// =============================================================================

func (p *JSPlugin) createNotifyAPI() map[string]interface{} {
	return map[string]interface{}{
		"send": func(title, msg string) {
			// 目前仅打印到日志，后续对接系统通知
			fmt.Printf("[NOTIFY][%s] %s: %s\n", p.name, title, msg)
		},
	}
}

// =============================================================================
// Route API (用于 Web API 代理)
// =============================================================================

func (p *JSPlugin) createRouteAPI() map[string]interface{} {
	return map[string]interface{}{
		"handle": p.apiHandle,
		"get":    func(path string, handler goja.Callable) { p.apiRegister("GET", path, handler) },
		"post":   func(path string, handler goja.Callable) { p.apiRegister("POST", path, handler) },
		"put":    func(path string, handler goja.Callable) { p.apiRegister("PUT", path, handler) },
		"delete": func(path string, handler goja.Callable) { p.apiRegister("DELETE", path, handler) },
	}
}

// apiHandle 注册 API 路由处理函数
func (p *JSPlugin) apiHandle(method, path string, handler goja.Callable) {
	p.apiRegister(method, path, handler)
}

// apiRegister 注册 API 路由
func (p *JSPlugin) apiRegister(method, path string, handler goja.Callable) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.apiHandlers[method] == nil {
		p.apiHandlers[method] = make(map[string]goja.Callable)
	}
	p.apiHandlers[method][path] = handler
	fmt.Printf("[JS:%s] Registered API: %s %s\n", p.name, method, path)
}

// HandleAPIRequest 处理 API 请求
func (p *JSPlugin) HandleAPIRequest(method, path, query string, headers map[string]string, body string) (int, map[string]string, string, error) {
	p.mu.Lock()
	handlers := p.apiHandlers[method]
	p.mu.Unlock()

	if handlers == nil {
		return 404, nil, `{"error":"method not allowed"}`, nil
	}

	// 查找匹配的路由
	var handler goja.Callable
	var matchedPath string

	for registeredPath, h := range handlers {
		if matchRoute(registeredPath, path) {
			handler = h
			matchedPath = registeredPath
			break
		}
	}

	if handler == nil {
		return 404, nil, `{"error":"route not found"}`, nil
	}

	// 构建请求对象
	reqObj := map[string]interface{}{
		"method":  method,
		"path":    path,
		"pattern": matchedPath,
		"query":   query,
		"headers": headers,
		"body":    body,
		"params":  extractParams(matchedPath, path),
	}

	// 调用处理函数
	result, err := handler(goja.Undefined(), p.vm.ToValue(reqObj))
	if err != nil {
		return 500, nil, fmt.Sprintf(`{"error":"%s"}`, err.Error()), nil
	}

	// 解析响应
	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return 200, nil, "", nil
	}

	respObj := result.ToObject(p.vm)
	status := 200
	if s := respObj.Get("status"); s != nil && !goja.IsUndefined(s) {
		status = int(s.ToInteger())
	}

	respHeaders := make(map[string]string)
	if h := respObj.Get("headers"); h != nil && !goja.IsUndefined(h) {
		hObj := h.ToObject(p.vm)
		for _, key := range hObj.Keys() {
			respHeaders[key] = hObj.Get(key).String()
		}
	}

	respBody := ""
	if b := respObj.Get("body"); b != nil && !goja.IsUndefined(b) {
		respBody = b.String()
	}

	return status, respHeaders, respBody, nil
}

// matchRoute 匹配路由 (支持简单的路径参数)
func matchRoute(pattern, path string) bool {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i, part := range patternParts {
		if strings.HasPrefix(part, ":") {
			continue // 路径参数，匹配任意值
		}
		if part != pathParts[i] {
			return false
		}
	}
	return true
}

// extractParams 提取路径参数
func extractParams(pattern, path string) map[string]string {
	params := make(map[string]string)
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	for i, part := range patternParts {
		if strings.HasPrefix(part, ":") && i < len(pathParts) {
			paramName := strings.TrimPrefix(part, ":")
			params[paramName] = pathParts[i]
		}
	}
	return params
}
