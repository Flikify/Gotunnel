package script

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
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
	config   map[string]string
	sandbox  *Sandbox
	running  bool
	mu       sync.Mutex
}

// NewJSPlugin 从 JS 源码创建插件
func NewJSPlugin(name, source string) (*JSPlugin, error) {
	p := &JSPlugin{
		name:    name,
		source:  source,
		vm:      goja.New(),
		sandbox: DefaultSandbox(),
	}

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
	// 注入基础 API
	p.vm.Set("log", p.jsLog)
	p.vm.Set("config", p.jsGetConfig)

	// 注入文件 API
	p.vm.Set("fs", p.createFsAPI())

	// 注入 HTTP API
	p.vm.Set("http", p.createHttpAPI())

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
	p.vm.Set("config", config)
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
