package router

import (
	"encoding/json"
	"net/http"

	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/pkg/protocol"
)

// ClientStatus 客户端状态
type ClientStatus struct {
	ID        string `json:"id"`
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
}

// AppInterface 应用接口
type AppInterface interface {
	GetClientStore() db.ClientStore
	GetServer() ServerInterface
}

// APIHandler API处理器
type APIHandler struct {
	clientStore db.ClientStore
	server      ServerInterface
}

// RegisterRoutes 注册所有 API 路由
func RegisterRoutes(r *Router, app AppInterface) {
	h := &APIHandler{
		clientStore: app.GetClientStore(),
		server:      app.GetServer(),
	}

	api := r.Group("/api")
	api.HandleFunc("/status", h.handleStatus)
	api.HandleFunc("/clients", h.handleClients)
	api.HandleFunc("/client/", h.handleClient)
	api.HandleFunc("/config/reload", h.handleReload)
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
		cs := ClientStatus{ID: c.ID, RuleCount: len(c.Rules)}
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

func (h *APIHandler) getClient(rw http.ResponseWriter, clientID string) {
	client, err := h.clientStore.GetClient(clientID)
	if err != nil {
		http.Error(rw, "client not found", http.StatusNotFound)
		return
	}
	online, lastPing := h.server.GetClientStatus(clientID)
	h.jsonResponse(rw, map[string]interface{}{
		"id": client.ID, "rules": client.Rules,
		"online": online, "last_ping": lastPing,
	})
}

func (h *APIHandler) updateClient(rw http.ResponseWriter, r *http.Request, clientID string) {
	var req struct {
		Rules []protocol.ProxyRule `json:"rules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	exists, _ := h.clientStore.ClientExists(clientID)
	if !exists {
		http.Error(rw, "client not found", http.StatusNotFound)
		return
	}

	client := &db.Client{ID: clientID, Rules: req.Rules}
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
