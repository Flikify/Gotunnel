package router

import (
	"encoding/json"
	"net/http"
	"regexp"

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
	ID        string `json:"id"`
	Nickname  string `json:"nickname,omitempty"`
	Online    bool   `json:"online"`
	LastPing  string `json:"last_ping,omitempty"`
	RuleCount int    `json:"rule_count"`
}

// ServerInterface 服务端接口
type ServerInterface interface {
	GetClientStatus(clientID string) (online bool, lastPing string)
	GetAllClientStatus() map[string]struct {
		Online   bool
		LastPing string
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
}

// PluginInfo 插件信息
type PluginInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Enabled     bool   `json:"enabled"`
}

// AppInterface 应用接口
type AppInterface interface {
	GetClientStore() db.ClientStore
	GetServer() ServerInterface
	GetConfig() *config.ServerConfig
	GetConfigPath() string
	SaveConfig() error
}

// APIHandler API处理器
type APIHandler struct {
	clientStore db.ClientStore
	server      ServerInterface
	app         AppInterface
}

// RegisterRoutes 注册所有 API 路由
func RegisterRoutes(r *Router, app AppInterface) {
	h := &APIHandler{
		clientStore: app.GetClientStore(),
		server:      app.GetServer(),
		app:         app,
	}

	api := r.Group("/api")
	api.HandleFunc("/status", h.handleStatus)
	api.HandleFunc("/clients", h.handleClients)
	api.HandleFunc("/client/", h.handleClient)
	api.HandleFunc("/config", h.handleConfig)
	api.HandleFunc("/config/reload", h.handleReload)
	api.HandleFunc("/plugins", h.handlePlugins)
	api.HandleFunc("/plugin/", h.handlePlugin)
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
	online, lastPing := h.server.GetClientStatus(clientID)
	h.jsonResponse(rw, map[string]interface{}{
		"id": client.ID, "nickname": client.Nickname, "rules": client.Rules,
		"online": online, "last_ping": lastPing,
	})
}

func (h *APIHandler) updateClient(rw http.ResponseWriter, r *http.Request, clientID string) {
	var req struct {
		Nickname string               `json:"nickname"`
		Rules    []protocol.ProxyRule `json:"rules"`
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

	online, _ := h.server.GetClientStatus(clientID)
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

	online, _ := h.server.GetClientStatus(clientID)
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
