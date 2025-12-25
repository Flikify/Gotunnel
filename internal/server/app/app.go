package app

import (
	"embed"
	"io"
	"io/fs"
	"log"
	"net/http"

	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/router"
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
	ClientStore db.ClientStore
	Server      router.ServerInterface
}

// NewWebServer 创建Web服务
func NewWebServer(cs db.ClientStore, srv router.ServerInterface) *WebServer {
	return &WebServer{
		ClientStore: cs,
		Server:      srv,
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

	handler := &authMiddleware{username, password, r.Handler()}
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

// GetClientStore 获取客户端存储
func (w *WebServer) GetClientStore() db.ClientStore {
	return w.ClientStore
}

// GetServer 获取服务端接口
func (w *WebServer) GetServer() router.ServerInterface {
	return w.Server
}
