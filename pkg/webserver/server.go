package webserver

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"sync"

	"github.com/gotunnel/pkg/config"
)

//go:embed dist/*
var staticFiles embed.FS

// spaHandler SPA路由处理器
type spaHandler struct {
	fs http.FileSystem
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	f, err := h.fs.Open(path)
	if err != nil {
		f, err = h.fs.Open("index.html")
		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
	}
	defer f.Close()

	stat, _ := f.Stat()
	if stat.IsDir() {
		f, err = h.fs.Open(path + "/index.html")
		if err != nil {
			f, _ = h.fs.Open("index.html")
		}
	}
	http.ServeContent(w, r, path, stat.ModTime(), f.(io.ReadSeeker))
}

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
}

// WebServer Web控制台服务
type WebServer struct {
	config     *config.ServerConfig
	configPath string
	server     ServerInterface
	mu         sync.RWMutex
}

// NewWebServer 创建Web服务
func NewWebServer(cfg *config.ServerConfig, configPath string, srv ServerInterface) *WebServer {
	return &WebServer{
		config:     cfg,
		configPath: configPath,
		server:     srv,
	}
}

// Run 启动Web服务
func (w *WebServer) Run(addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/status", w.handleStatus)
	mux.HandleFunc("/api/clients", w.handleClients)
	mux.HandleFunc("/api/client/", w.handleClient)
	mux.HandleFunc("/api/config/reload", w.handleReload)

	staticFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		return err
	}
	mux.Handle("/", spaHandler{fs: http.FS(staticFS)})

	log.Printf("[Web] Console listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

// handleStatus 获取服务状态
func (w *WebServer) handleStatus(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.mu.RLock()
	defer w.mu.RUnlock()

	status := map[string]interface{}{
		"server": map[string]interface{}{
			"bind_addr": w.config.Server.BindAddr,
			"bind_port": w.config.Server.BindPort,
		},
		"client_count": len(w.config.Clients),
	}
	w.jsonResponse(rw, status)
}

// handleClients 获取所有客户端
func (w *WebServer) handleClients(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.getClients(rw)
	case http.MethodPost:
		w.addClient(rw, r)
	default:
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (w *WebServer) getClients(rw http.ResponseWriter) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var clients []ClientStatus
	statusMap := w.server.GetAllClientStatus()

	for _, c := range w.config.Clients {
		cs := ClientStatus{ID: c.ID, RuleCount: len(c.Rules)}
		if s, ok := statusMap[c.ID]; ok {
			cs.Online = s.Online
			cs.LastPing = s.LastPing
		}
		clients = append(clients, cs)
	}
	w.jsonResponse(rw, clients)
}

func (w *WebServer) addClient(rw http.ResponseWriter, r *http.Request) {
	var client config.ClientConfig
	if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	if client.ID == "" {
		http.Error(rw, "client id required", http.StatusBadRequest)
		return
	}

	w.mu.Lock()
	for _, c := range w.config.Clients {
		if c.ID == client.ID {
			w.mu.Unlock()
			http.Error(rw, "client already exists", http.StatusConflict)
			return
		}
	}
	w.config.Clients = append(w.config.Clients, client)
	w.mu.Unlock()

	if err := w.saveConfig(); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	w.jsonResponse(rw, map[string]string{"status": "ok"})
}

func (w *WebServer) handleClient(rw http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Path[len("/api/client/"):]
	if clientID == "" {
		http.Error(rw, "client id required", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.getClient(rw, clientID)
	case http.MethodPut:
		w.updateClient(rw, r, clientID)
	case http.MethodDelete:
		w.deleteClient(rw, clientID)
	default:
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (w *WebServer) getClient(rw http.ResponseWriter, clientID string) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for _, c := range w.config.Clients {
		if c.ID == clientID {
			online, lastPing := w.server.GetClientStatus(clientID)
			w.jsonResponse(rw, map[string]interface{}{
				"id": c.ID, "rules": c.Rules,
				"online": online, "last_ping": lastPing,
			})
			return
		}
	}
	http.Error(rw, "client not found", http.StatusNotFound)
}

func (w *WebServer) updateClient(rw http.ResponseWriter, r *http.Request, clientID string) {
	var client config.ClientConfig
	if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	w.mu.Lock()
	found := false
	for i, c := range w.config.Clients {
		if c.ID == clientID {
			client.ID = clientID
			w.config.Clients[i] = client
			found = true
			break
		}
	}
	w.mu.Unlock()

	if !found {
		http.Error(rw, "client not found", http.StatusNotFound)
		return
	}
	if err := w.saveConfig(); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	w.jsonResponse(rw, map[string]string{"status": "ok"})
}

func (w *WebServer) deleteClient(rw http.ResponseWriter, clientID string) {
	w.mu.Lock()
	found := false
	for i, c := range w.config.Clients {
		if c.ID == clientID {
			w.config.Clients = append(w.config.Clients[:i], w.config.Clients[i+1:]...)
			found = true
			break
		}
	}
	w.mu.Unlock()

	if !found {
		http.Error(rw, "client not found", http.StatusNotFound)
		return
	}
	if err := w.saveConfig(); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	w.jsonResponse(rw, map[string]string{"status": "ok"})
}

func (w *WebServer) handleReload(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := w.server.ReloadConfig(); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	w.jsonResponse(rw, map[string]string{"status": "ok"})
}

func (w *WebServer) saveConfig() error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return config.SaveServerConfig(w.configPath, w.config)
}

func (w *WebServer) jsonResponse(rw http.ResponseWriter, data interface{}) {
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(data)
}

// RunWithAuth 启动带认证的Web服务
func (w *WebServer) RunWithAuth(addr, username, password string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/status", w.handleStatus)
	mux.HandleFunc("/api/clients", w.handleClients)
	mux.HandleFunc("/api/client/", w.handleClient)
	mux.HandleFunc("/api/config/reload", w.handleReload)

	staticFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		return fmt.Errorf("failed to load static files: %v", err)
	}
	mux.Handle("/", spaHandler{fs: http.FS(staticFS)})

	handler := &authMiddleware{username, password, mux}
	log.Printf("[Web] Console listening on %s (auth enabled)", addr)
	return http.ListenAndServe(addr, handler)
}

type authMiddleware struct {
	username, password string
	handler            http.Handler
}

func (a *authMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, pass, ok := r.BasicAuth()
	if !ok || user != a.username || pass != a.password {
		w.Header().Set("WWW-Authenticate", `Basic realm="GoTunnel"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	a.handler.ServeHTTP(w, r)
}
