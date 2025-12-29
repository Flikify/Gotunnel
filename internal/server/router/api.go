package router

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/gotunnel/internal/server/config"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/pkg/protocol"
)

// 客户端 ID 验证规则
var clientIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// validateClientID 验证客户端 ID 格式
func validateClientID(id string) bool {
	return clientIDRegex.MatchString(id)
}

// ClientStatus 客户端状态
type ClientStatus struct {
	ID         string `json:"id"`
	Nickname   string `json:"nickname,omitempty"`
	Online     bool   `json:"online"`
	LastPing   string `json:"last_ping,omitempty"`
	RemoteAddr string `json:"remote_addr,omitempty"`
	RuleCount  int    `json:"rule_count"`
}

// ServerInterface 服务端接口
type ServerInterface interface {
	GetClientStatus(clientID string) (online bool, lastPing string, remoteAddr string)
	GetAllClientStatus() map[string]struct {
		Online     bool
		LastPing   string
		RemoteAddr string
	}
	ReloadConfig() error
	GetBindAddr() string
	GetBindPort() int
	// 客户端控制
	PushConfigToClient(clientID string) error
	DisconnectClient(clientID string) error
	GetPluginList() []PluginInfo
	EnablePlugin(name string) error
	DisablePlugin(name string) error
	InstallPluginsToClient(clientID string, plugins []string) error
	// 插件配置
	GetPluginConfigSchema(name string) ([]ConfigField, error)
	SyncPluginConfigToClient(clientID string, pluginName string, config map[string]string) error
	// JS 插件
	InstallJSPluginToClient(clientID string, req JSPluginInstallRequest) error
}

// JSPluginInstallRequest JS 插件安装请求
type JSPluginInstallRequest struct {
	PluginName string            `json:"plugin_name"`
	Source     string            `json:"source"`
	Signature  string            `json:"signature"`
	RuleName   string            `json:"rule_name"`
	RemotePort int               `json:"remote_port"`
	Config     map[string]string `json:"config"`
	AutoStart  bool              `json:"auto_start"`
}

// ConfigField 配置字段（从 plugin 包导出）
type ConfigField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Default     string   `json:"default,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Options     []string `json:"options,omitempty"`
	Description string   `json:"description,omitempty"`
}

// RuleSchema 规则表单模式
type RuleSchema struct {
	NeedsLocalAddr bool          `json:"needs_local_addr"`
	ExtraFields    []ConfigField `json:"extra_fields,omitempty"`
}

// PluginInfo 插件信息
type PluginInfo struct {
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Source      string      `json:"source"`
	Icon        string      `json:"icon,omitempty"`
	Enabled     bool        `json:"enabled"`
	RuleSchema  *RuleSchema `json:"rule_schema,omitempty"`
}

// AppInterface 应用接口
type AppInterface interface {
	GetClientStore() db.ClientStore
	GetServer() ServerInterface
	GetConfig() *config.ServerConfig
	GetConfigPath() string
	SaveConfig() error
	GetJSPluginStore() db.JSPluginStore
}

// APIHandler API处理器
type APIHandler struct {
	clientStore   db.ClientStore
	server        ServerInterface
	app           AppInterface
	jsPluginStore db.JSPluginStore
}

// RegisterRoutes 注册所有 API 路由
func RegisterRoutes(r *Router, app AppInterface) {
	h := &APIHandler{
		clientStore:   app.GetClientStore(),
		server:        app.GetServer(),
		app:           app,
		jsPluginStore: app.GetJSPluginStore(),
	}

	api := r.Group("/api")
	api.HandleFunc("/status", h.handleStatus)
	api.HandleFunc("/clients", h.handleClients)
	api.HandleFunc("/client/", h.handleClient)
	api.HandleFunc("/config", h.handleConfig)
	api.HandleFunc("/config/reload", h.handleReload)
	api.HandleFunc("/plugins", h.handlePlugins)
	api.HandleFunc("/plugin/", h.handlePlugin)
	api.HandleFunc("/store/plugins", h.handleStorePlugins)
	api.HandleFunc("/store/install", h.handleStoreInstall)
	api.HandleFunc("/client-plugin/", h.handleClientPlugin)
	api.HandleFunc("/js-plugin/", h.handleJSPlugin)
	api.HandleFunc("/js-plugins", h.handleJSPlugins)
}

func (h *APIHandler) handleStatus(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clients, _ := h.clientStore.GetAllClients()
	status := map[string]interface{}{
		"server": map[string]interface{}{
			"bind_addr": h.server.GetBindAddr(),
			"bind_port": h.server.GetBindPort(),
		},
		"client_count": len(clients),
	}
	h.jsonResponse(rw, status)
}

func (h *APIHandler) handleClients(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getClients(rw)
	case http.MethodPost:
		h.addClient(rw, r)
	default:
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) getClients(rw http.ResponseWriter) {
	clients, err := h.clientStore.GetAllClients()
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	statusMap := h.server.GetAllClientStatus()
	var result []ClientStatus
	for _, c := range clients {
		cs := ClientStatus{ID: c.ID, Nickname: c.Nickname, RuleCount: len(c.Rules)}
		if s, ok := statusMap[c.ID]; ok {
			cs.Online = s.Online
			cs.LastPing = s.LastPing
			cs.RemoteAddr = s.RemoteAddr
		}
		result = append(result, cs)
	}
	h.jsonResponse(rw, result)
}

func (h *APIHandler) addClient(rw http.ResponseWriter, r *http.Request) {
	var req struct {
		ID    string               `json:"id"`
		Rules []protocol.ProxyRule `json:"rules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		http.Error(rw, "client id required", http.StatusBadRequest)
		return
	}
	if !validateClientID(req.ID) {
		http.Error(rw, "invalid client id: must be 1-64 alphanumeric characters, underscore or hyphen", http.StatusBadRequest)
		return
	}

	exists, _ := h.clientStore.ClientExists(req.ID)
	if exists {
		http.Error(rw, "client already exists", http.StatusConflict)
		return
	}

	client := &db.Client{ID: req.ID, Rules: req.Rules}
	if err := h.clientStore.CreateClient(client); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

func (h *APIHandler) handleClient(rw http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Path[len("/api/client/"):]
	if clientID == "" {
		http.Error(rw, "client id required", http.StatusBadRequest)
		return
	}

	// 处理子路径操作
	if idx := len(clientID) - 1; idx > 0 {
		if clientID[idx] == '/' {
			clientID = clientID[:idx]
		}
	}

	// 检查是否是特殊操作
	parts := splitPath(clientID)
	if len(parts) == 2 {
		clientID = parts[0]
		action := parts[1]
		switch action {
		case "push":
			h.pushConfigToClient(rw, r, clientID)
			return
		case "disconnect":
			h.disconnectClient(rw, r, clientID)
			return
		case "install-plugins":
			h.installPluginsToClient(rw, r, clientID)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		h.getClient(rw, clientID)
	case http.MethodPut:
		h.updateClient(rw, r, clientID)
	case http.MethodDelete:
		h.deleteClient(rw, clientID)
	default:
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// splitPath 分割路径
func splitPath(path string) []string {
	for i, c := range path {
		if c == '/' {
			return []string{path[:i], path[i+1:]}
		}
	}
	return []string{path}
}

func (h *APIHandler) getClient(rw http.ResponseWriter, clientID string) {
	client, err := h.clientStore.GetClient(clientID)
	if err != nil {
		http.Error(rw, "client not found", http.StatusNotFound)
		return
	}
	online, lastPing, remoteAddr := h.server.GetClientStatus(clientID)
	h.jsonResponse(rw, map[string]interface{}{
		"id": client.ID, "nickname": client.Nickname, "rules": client.Rules,
		"plugins": client.Plugins, "online": online, "last_ping": lastPing,
		"remote_addr": remoteAddr,
	})
}

func (h *APIHandler) updateClient(rw http.ResponseWriter, r *http.Request, clientID string) {
	var req struct {
		Nickname string               `json:"nickname"`
		Rules    []protocol.ProxyRule `json:"rules"`
		Plugins  []db.ClientPlugin    `json:"plugins"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	client, err := h.clientStore.GetClient(clientID)
	if err != nil {
		http.Error(rw, "client not found", http.StatusNotFound)
		return
	}

	client.Nickname = req.Nickname
	client.Rules = req.Rules
	if req.Plugins != nil {
		client.Plugins = req.Plugins
	}
	if err := h.clientStore.UpdateClient(client); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

func (h *APIHandler) deleteClient(rw http.ResponseWriter, clientID string) {
	exists, _ := h.clientStore.ClientExists(clientID)
	if !exists {
		http.Error(rw, "client not found", http.StatusNotFound)
		return
	}

	if err := h.clientStore.DeleteClient(clientID); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

func (h *APIHandler) handleConfig(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getConfig(rw)
	case http.MethodPut:
		h.updateConfig(rw, r)
	default:
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) getConfig(rw http.ResponseWriter) {
	cfg := h.app.GetConfig()
	// Token 脱敏处理，只显示前4位
	maskedToken := cfg.Server.Token
	if len(maskedToken) > 4 {
		maskedToken = maskedToken[:4] + "****"
	}
	h.jsonResponse(rw, map[string]interface{}{
		"server": map[string]interface{}{
			"bind_addr":         cfg.Server.BindAddr,
			"bind_port":         cfg.Server.BindPort,
			"token":             maskedToken,
			"heartbeat_sec":     cfg.Server.HeartbeatSec,
			"heartbeat_timeout": cfg.Server.HeartbeatTimeout,
		},
		"web": map[string]interface{}{
			"enabled":   cfg.Web.Enabled,
			"bind_addr": cfg.Web.BindAddr,
			"bind_port": cfg.Web.BindPort,
			"username":  cfg.Web.Username,
			"password":  "****",
		},
	})
}

func (h *APIHandler) updateConfig(rw http.ResponseWriter, r *http.Request) {
	var req struct {
		Server *struct {
			BindAddr         string `json:"bind_addr"`
			BindPort         int    `json:"bind_port"`
			Token            string `json:"token"`
			HeartbeatSec     int    `json:"heartbeat_sec"`
			HeartbeatTimeout int    `json:"heartbeat_timeout"`
		} `json:"server"`
		Web *struct {
			Enabled  bool   `json:"enabled"`
			BindAddr string `json:"bind_addr"`
			BindPort int    `json:"bind_port"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"web"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	cfg := h.app.GetConfig()

	// 更新 Server 配置
	if req.Server != nil {
		if req.Server.BindAddr != "" {
			cfg.Server.BindAddr = req.Server.BindAddr
		}
		if req.Server.BindPort > 0 {
			cfg.Server.BindPort = req.Server.BindPort
		}
		if req.Server.Token != "" {
			cfg.Server.Token = req.Server.Token
		}
		if req.Server.HeartbeatSec > 0 {
			cfg.Server.HeartbeatSec = req.Server.HeartbeatSec
		}
		if req.Server.HeartbeatTimeout > 0 {
			cfg.Server.HeartbeatTimeout = req.Server.HeartbeatTimeout
		}
	}

	// 更新 Web 配置
	if req.Web != nil {
		cfg.Web.Enabled = req.Web.Enabled
		if req.Web.BindAddr != "" {
			cfg.Web.BindAddr = req.Web.BindAddr
		}
		if req.Web.BindPort > 0 {
			cfg.Web.BindPort = req.Web.BindPort
		}
		cfg.Web.Username = req.Web.Username
		cfg.Web.Password = req.Web.Password
	}

	if err := h.app.SaveConfig(); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

func (h *APIHandler) handleReload(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.server.ReloadConfig(); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

func (h *APIHandler) jsonResponse(rw http.ResponseWriter, data interface{}) {
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(data)
}

// pushConfigToClient 推送配置到客户端
func (h *APIHandler) pushConfigToClient(rw http.ResponseWriter, r *http.Request, clientID string) {
	if r.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	online, _, _ := h.server.GetClientStatus(clientID)
	if !online {
		http.Error(rw, "client not online", http.StatusBadRequest)
		return
	}

	if err := h.server.PushConfigToClient(clientID); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

// disconnectClient 断开客户端连接
func (h *APIHandler) disconnectClient(rw http.ResponseWriter, r *http.Request, clientID string) {
	if r.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.server.DisconnectClient(clientID); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

// handlePlugins 处理插件列表
func (h *APIHandler) handlePlugins(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	plugins := h.server.GetPluginList()
	h.jsonResponse(rw, plugins)
}

// handlePlugin 处理单个插件操作
func (h *APIHandler) handlePlugin(rw http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/api/plugin/"):]
	if path == "" {
		http.Error(rw, "plugin name required", http.StatusBadRequest)
		return
	}

	parts := splitPath(path)
	pluginName := parts[0]

	if len(parts) == 2 {
		action := parts[1]
		switch action {
		case "enable":
			h.enablePlugin(rw, r, pluginName)
			return
		case "disable":
			h.disablePlugin(rw, r, pluginName)
			return
		}
	}

	http.Error(rw, "invalid action", http.StatusBadRequest)
}

func (h *APIHandler) enablePlugin(rw http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.server.EnablePlugin(name); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

func (h *APIHandler) disablePlugin(rw http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.server.DisablePlugin(name); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

// installPluginsToClient 安装插件到客户端
func (h *APIHandler) installPluginsToClient(rw http.ResponseWriter, r *http.Request, clientID string) {
	if r.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	online, _, _ := h.server.GetClientStatus(clientID)
	if !online {
		http.Error(rw, "client not online", http.StatusBadRequest)
		return
	}

	var req struct {
		Plugins []string `json:"plugins"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Plugins) == 0 {
		http.Error(rw, "no plugins specified", http.StatusBadRequest)
		return
	}

	if err := h.server.InstallPluginsToClient(clientID, req.Plugins); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

// StorePluginInfo 扩展商店插件信息
type StorePluginInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Icon        string `json:"icon,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
}

// StorePluginInstallRequest 从商店安装插件的请求
type StorePluginInstallRequest struct {
	PluginName  string `json:"plugin_name"`
	DownloadURL string `json:"download_url"`
	ClientID    string `json:"client_id"`
}

// handleStorePlugins 处理扩展商店插件列表
func (h *APIHandler) handleStorePlugins(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg := h.app.GetConfig()
	storeURL := config.OfficialPluginStoreURL
	_ = cfg // 保留以便未来扩展

	// 从远程URL获取插件列表
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(storeURL)
	if err != nil {
		http.Error(rw, "Failed to fetch store: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(rw, "Store returned error", http.StatusBadGateway)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(rw, "Failed to read response", http.StatusInternalServerError)
		return
	}

	var plugins []StorePluginInfo
	if err := json.Unmarshal(body, &plugins); err != nil {
		http.Error(rw, "Invalid store format", http.StatusBadGateway)
		return
	}

	h.jsonResponse(rw, map[string]interface{}{
		"plugins":   plugins,
		"store_url": storeURL,
	})
}

// handleStoreInstall 从商店安装插件到客户端
func (h *APIHandler) handleStoreInstall(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StorePluginInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	if req.PluginName == "" || req.DownloadURL == "" || req.ClientID == "" {
		http.Error(rw, "plugin_name, download_url and client_id required", http.StatusBadRequest)
		return
	}

	// 检查客户端是否在线
	online, _, _ := h.server.GetClientStatus(req.ClientID)
	if !online {
		http.Error(rw, "client not online", http.StatusBadRequest)
		return
	}

	// 下载插件
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(req.DownloadURL)
	if err != nil {
		http.Error(rw, "Failed to download plugin: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(rw, "Plugin download failed with status: "+resp.Status, http.StatusBadGateway)
		return
	}

	source, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(rw, "Failed to read plugin: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 安装到客户端
	installReq := JSPluginInstallRequest{
		PluginName: req.PluginName,
		Source:     string(source),
		RuleName:   req.PluginName,
		AutoStart:  true,
	}

	if err := h.server.InstallJSPluginToClient(req.ClientID, installReq); err != nil {
		http.Error(rw, "Failed to install plugin: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonResponse(rw, map[string]interface{}{
		"status": "ok",
		"plugin": req.PluginName,
		"client": req.ClientID,
	})
}

// handleClientPlugin 处理客户端插件配置
// 路由: /api/client-plugin/{clientID}/{pluginName}/config
func (h *APIHandler) handleClientPlugin(rw http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/api/client-plugin/"):]
	if path == "" {
		http.Error(rw, "client id required", http.StatusBadRequest)
		return
	}

	// 解析路径: clientID/pluginName/action
	parts := splitPathMulti(path)
	if len(parts) < 3 {
		http.Error(rw, "invalid path, expected: /api/client-plugin/{clientID}/{pluginName}/config", http.StatusBadRequest)
		return
	}

	clientID := parts[0]
	pluginName := parts[1]
	action := parts[2]

	if action != "config" {
		http.Error(rw, "invalid action", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getClientPluginConfig(rw, clientID, pluginName)
	case http.MethodPut:
		h.updateClientPluginConfig(rw, r, clientID, pluginName)
	default:
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// splitPathMulti 分割路径为多个部分
func splitPathMulti(path string) []string {
	var parts []string
	start := 0
	for i, c := range path {
		if c == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

// getClientPluginConfig 获取客户端插件配置
func (h *APIHandler) getClientPluginConfig(rw http.ResponseWriter, clientID, pluginName string) {
	client, err := h.clientStore.GetClient(clientID)
	if err != nil {
		http.Error(rw, "client not found", http.StatusNotFound)
		return
	}

	// 获取插件配置模式
	schema, err := h.server.GetPluginConfigSchema(pluginName)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusNotFound)
		return
	}

	// 查找客户端的插件配置
	var config map[string]string
	for _, p := range client.Plugins {
		if p.Name == pluginName {
			config = p.Config
			break
		}
	}
	if config == nil {
		config = make(map[string]string)
	}

	h.jsonResponse(rw, map[string]interface{}{
		"plugin_name": pluginName,
		"schema":      schema,
		"config":      config,
	})
}

// updateClientPluginConfig 更新客户端插件配置
func (h *APIHandler) updateClientPluginConfig(rw http.ResponseWriter, r *http.Request, clientID, pluginName string) {
	var req struct {
		Config map[string]string `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	client, err := h.clientStore.GetClient(clientID)
	if err != nil {
		http.Error(rw, "client not found", http.StatusNotFound)
		return
	}

	// 更新插件配置
	found := false
	for i, p := range client.Plugins {
		if p.Name == pluginName {
			client.Plugins[i].Config = req.Config
			found = true
			break
		}
	}

	if !found {
		http.Error(rw, "plugin not installed on client", http.StatusNotFound)
		return
	}

	// 保存到数据库
	if err := h.clientStore.UpdateClient(client); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	// 如果客户端在线，同步配置
	online, _, _ := h.server.GetClientStatus(clientID)
	if online {
		if err := h.server.SyncPluginConfigToClient(clientID, pluginName, req.Config); err != nil {
			// 配置已保存，但同步失败，返回警告
			h.jsonResponse(rw, map[string]interface{}{
				"status":  "partial",
				"message": "config saved but sync failed: " + err.Error(),
			})
			return
		}
	}

	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

// handleJSPlugin 处理单个 JS 插件操作
// GET/PUT/DELETE /api/js-plugin/{name}
// POST /api/js-plugin/{name}/push/{clientID}
func (h *APIHandler) handleJSPlugin(rw http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/api/js-plugin/"):]
	if path == "" {
		http.Error(rw, "plugin name required", http.StatusBadRequest)
		return
	}

	parts := splitPathMulti(path)

	// POST /api/js-plugin/{name}/push/{clientID}
	if len(parts) == 3 && parts[1] == "push" {
		if r.Method != http.MethodPost {
			http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.pushJSPluginToClient(rw, parts[0], parts[2])
		return
	}

	// GET/PUT/DELETE /api/js-plugin/{name}
	pluginName := parts[0]
	switch r.Method {
	case http.MethodGet:
		h.getJSPlugin(rw, pluginName)
	case http.MethodPut:
		h.updateJSPlugin(rw, r, pluginName)
	case http.MethodDelete:
		h.deleteJSPlugin(rw, pluginName)
	default:
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// installJSPluginToClient 安装 JS 插件到客户端
func (h *APIHandler) installJSPluginToClient(rw http.ResponseWriter, r *http.Request, clientID string) {
	online, _, _ := h.server.GetClientStatus(clientID)
	if !online {
		http.Error(rw, "client not online", http.StatusBadRequest)
		return
	}

	var req JSPluginInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	if req.PluginName == "" || req.Source == "" {
		http.Error(rw, "plugin_name and source required", http.StatusBadRequest)
		return
	}

	if err := h.server.InstallJSPluginToClient(clientID, req); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonResponse(rw, map[string]interface{}{
		"status": "ok",
		"plugin": req.PluginName,
	})
}

// handleJSPlugins 处理 JS 插件列表和创建
// GET /api/js-plugins - 获取所有 JS 插件
// POST /api/js-plugins - 创建新 JS 插件
func (h *APIHandler) handleJSPlugins(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getJSPlugins(rw)
	case http.MethodPost:
		h.createJSPlugin(rw, r)
	default:
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) getJSPlugins(rw http.ResponseWriter) {
	plugins, err := h.jsPluginStore.GetAllJSPlugins()
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	if plugins == nil {
		plugins = []db.JSPlugin{}
	}
	h.jsonResponse(rw, plugins)
}

func (h *APIHandler) createJSPlugin(rw http.ResponseWriter, r *http.Request) {
	var req db.JSPlugin
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Source == "" {
		http.Error(rw, "name and source required", http.StatusBadRequest)
		return
	}

	req.Enabled = true
	if err := h.jsPluginStore.SaveJSPlugin(&req); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

func (h *APIHandler) getJSPlugin(rw http.ResponseWriter, name string) {
	p, err := h.jsPluginStore.GetJSPlugin(name)
	if err != nil {
		http.Error(rw, "plugin not found", http.StatusNotFound)
		return
	}
	h.jsonResponse(rw, p)
}

func (h *APIHandler) updateJSPlugin(rw http.ResponseWriter, r *http.Request, name string) {
	var req db.JSPlugin
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	req.Name = name
	if err := h.jsPluginStore.SaveJSPlugin(&req); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

func (h *APIHandler) deleteJSPlugin(rw http.ResponseWriter, name string) {
	if err := h.jsPluginStore.DeleteJSPlugin(name); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonResponse(rw, map[string]string{"status": "ok"})
}

// pushJSPluginToClient 推送 JS 插件到指定客户端
func (h *APIHandler) pushJSPluginToClient(rw http.ResponseWriter, pluginName, clientID string) {
	// 检查客户端是否在线
	online, _, _ := h.server.GetClientStatus(clientID)
	if !online {
		http.Error(rw, "client not online", http.StatusBadRequest)
		return
	}

	// 获取插件
	p, err := h.jsPluginStore.GetJSPlugin(pluginName)
	if err != nil {
		http.Error(rw, "plugin not found", http.StatusNotFound)
		return
	}

	if !p.Enabled {
		http.Error(rw, "plugin is disabled", http.StatusBadRequest)
		return
	}

	// 推送到客户端
	req := JSPluginInstallRequest{
		PluginName: p.Name,
		Source:     p.Source,
		Signature:  p.Signature,
		RuleName:   p.Name,
		Config:     p.Config,
		AutoStart:  p.AutoStart,
	}

	if err := h.server.InstallJSPluginToClient(clientID, req); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonResponse(rw, map[string]string{"status": "ok", "plugin": pluginName, "client": clientID})
}
