package app

import (
	"embed"
	"io"
	"io/fs"
	"log"
	"net/http"

	"github.com/gotunnel/internal/server/config"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/router"
	"github.com/gotunnel/pkg/auth"
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

// WebServer Web控制台服务
type WebServer struct {
	ClientStore   db.ClientStore
	Server        router.ServerInterface
	Config        *config.ServerConfig
	ConfigPath    string
	JSPluginStore db.JSPluginStore
}

// NewWebServer 创建Web服务
func NewWebServer(cs db.ClientStore, srv router.ServerInterface, cfg *config.ServerConfig, cfgPath string, jsStore db.JSPluginStore) *WebServer {
	return &WebServer{
		ClientStore:   cs,
		Server:        srv,
		Config:        cfg,
		ConfigPath:    cfgPath,
		JSPluginStore: jsStore,
	}
}

// Run 启动Web服务
func (w *WebServer) Run(addr string) error {
	r := router.New()
	router.RegisterRoutes(r, w)

	staticFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		return err
	}
	r.Handle("/", spaHandler{fs: http.FS(staticFS)})

	log.Printf("[Web] Console listening on %s", addr)
	return http.ListenAndServe(addr, r.Handler())
}

// RunWithAuth 启动带认证的Web服务
func (w *WebServer) RunWithAuth(addr, username, password string) error {
	r := router.New()
	router.RegisterRoutes(r, w)

	staticFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		return err
	}
	r.Handle("/", spaHandler{fs: http.FS(staticFS)})

	auth := &router.AuthConfig{Username: username, Password: password}
	handler := router.BasicAuthMiddleware(auth, r.Handler())
	log.Printf("[Web] Console listening on %s (auth enabled)", addr)
	return http.ListenAndServe(addr, handler)
}

// RunWithJWT 启动带 JWT 认证的 Web 服务
func (w *WebServer) RunWithJWT(addr, username, password, jwtSecret string) error {
	r := router.New()

	// JWT 认证器
	jwtAuth := auth.NewJWTAuth(jwtSecret, 24) // 24小时过期

	// 注册认证路由（不需要认证）
	authHandler := router.NewAuthHandler(username, password, jwtAuth)
	router.RegisterAuthRoutes(r, authHandler)

	// 注册业务路由
	router.RegisterRoutes(r, w)

	// 静态文件
	staticFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		return err
	}
	r.Handle("/", spaHandler{fs: http.FS(staticFS)})

	// JWT 中间件，只对 /api/ 路径进行认证（排除 /api/auth/）
	skipPaths := []string{"/api/auth/"}
	handler := router.JWTMiddleware(jwtAuth, skipPaths, r.Handler())

	log.Printf("[Web] Console listening on %s (JWT auth enabled)", addr)
	return http.ListenAndServe(addr, handler)
}

// GetClientStore 获取客户端存储
func (w *WebServer) GetClientStore() db.ClientStore {
	return w.ClientStore
}

// GetServer 获取服务端接口
func (w *WebServer) GetServer() router.ServerInterface {
	return w.Server
}

// GetConfig 获取配置
func (w *WebServer) GetConfig() *config.ServerConfig {
	return w.Config
}

// GetConfigPath 获取配置文件路径
func (w *WebServer) GetConfigPath() string {
	return w.ConfigPath
}

// SaveConfig 保存配置
func (w *WebServer) SaveConfig() error {
	return config.SaveServerConfig(w.ConfigPath, w.Config)
}

// GetJSPluginStore 获取 JS 插件存储
func (w *WebServer) GetJSPluginStore() db.JSPluginStore {
	return w.JSPluginStore
}
