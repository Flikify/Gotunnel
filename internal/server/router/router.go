package router

import (
	"io"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/gotunnel/internal/server/router/handler"
	"github.com/gotunnel/internal/server/router/middleware"
	"github.com/gotunnel/pkg/auth"
)

// GinRouter Gin 路由管理器
type GinRouter struct {
	Engine *gin.Engine
}

// New 创建 Gin 路由管理器
func New() *GinRouter {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	return &GinRouter{Engine: engine}
}

// Handler 返回 http.Handler
func (r *GinRouter) Handler() http.Handler {
	return r.Engine
}

// SetupRoutes 配置所有路由
func (r *GinRouter) SetupRoutes(app handler.AppInterface, jwtAuth *auth.JWTAuth, username, password string) {
	engine := r.Engine

	// 全局中间件
	engine.Use(middleware.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())

	// Swagger 文档
	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 认证路由 (无需 JWT)
	authHandler := handler.NewAuthHandler(username, password, jwtAuth)
	engine.POST("/api/auth/login", authHandler.Login)
	engine.GET("/api/auth/check", authHandler.Check)

	installHandler := handler.NewInstallHandler(app)
	engine.GET("/install.sh", installHandler.ServeShellScript)
	engine.GET("/install.ps1", installHandler.ServePowerShellScript)
	engine.GET("/install/client", installHandler.DownloadClient)

	// API 路由 (需要 JWT)
	api := engine.Group("/api")
	api.Use(middleware.JWTAuth(jwtAuth))
	{
		// 状态
		statusHandler := handler.NewStatusHandler(app)
		api.GET("/status", statusHandler.GetStatus)
		api.GET("/update/version", statusHandler.GetVersion)

		// 客户端管理
		clientHandler := handler.NewClientHandler(app)
		api.GET("/clients", clientHandler.List)
		api.POST("/clients", clientHandler.Create)
		api.GET("/client/:id", clientHandler.Get)
		api.PUT("/client/:id", clientHandler.Update)
		api.DELETE("/client/:id", clientHandler.Delete)
		api.POST("/client/:id/push", clientHandler.PushConfig)
		api.POST("/client/:id/disconnect", clientHandler.Disconnect)
		api.POST("/client/:id/restart", clientHandler.Restart)
		api.GET("/client/:id/system-stats", clientHandler.GetSystemStats)
		api.GET("/client/:id/screenshot", clientHandler.GetScreenshot)
		api.POST("/client/:id/shell", clientHandler.ExecuteShell)

		// 配置管理
		configHandler := handler.NewConfigHandler(app)
		api.GET("/config", configHandler.Get)
		api.PUT("/config", configHandler.Update)
		api.POST("/config/reload", configHandler.Reload)

		// 更新管理
		updateHandler := handler.NewUpdateHandler(app)
		api.GET("/update/check/server", updateHandler.CheckServer)
		api.GET("/update/check/client", updateHandler.CheckClient)
		api.POST("/update/apply/server", updateHandler.ApplyServer)
		api.POST("/update/apply/client", updateHandler.ApplyClient)

		// 日志管理
		logHandler := handler.NewLogHandler(app)
		api.GET("/client/:id/logs", logHandler.StreamLogs)

		// 流量统计
		trafficHandler := handler.NewTrafficHandler(app)
		api.GET("/traffic/stats", trafficHandler.GetStats)
		api.GET("/traffic/hourly", trafficHandler.GetHourly)

		// 安装命令生成
		api.POST("/install/generate", installHandler.GenerateInstallCommand)
	}
}

// SetupStaticFiles 配置静态文件处理
func (r *GinRouter) SetupStaticFiles(staticFS fs.FS) {
	// 使用 NoRoute 处理 SPA 路由
	r.Engine.NoRoute(gin.WrapH(&spaHandler{fs: http.FS(staticFS)}))
}

// spaHandler SPA 路由处理器
type spaHandler struct {
	fs http.FileSystem
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// API 请求不应该返回 SPA 页面
	if len(path) >= 4 && path[:4] == "/api" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"code":404,"message":"Not Found"}`))
		return
	}

	// 尝试打开请求的文件
	f, err := h.fs.Open(path)
	if err != nil {
		// 文件不存在时，检查是否是静态资源请求
		// 静态资源（js, css, 图片等）应该返回 404，而不是 index.html
		if isStaticAsset(path) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		// 其他路径返回 index.html（SPA 路由）
		f, err = h.fs.Open("index.html")
		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if stat.IsDir() {
		f.Close()
		f, err = h.fs.Open(path + "/index.html")
		if err != nil {
			f, _ = h.fs.Open("index.html")
		}
		stat, _ = f.Stat()
	}

	if seeker, ok := f.(io.ReadSeeker); ok {
		http.ServeContent(w, r, path, stat.ModTime(), seeker)
	}
}

// isStaticAsset 检查路径是否是静态资源
func isStaticAsset(path string) bool {
	staticExtensions := []string{
		".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico",
		".woff", ".woff2", ".ttf", ".eot", ".map", ".json",
	}
	for _, ext := range staticExtensions {
		if len(path) > len(ext) && path[len(path)-len(ext):] == ext {
			return true
		}
	}
	return false
}

// Re-export types from handler package for backward compatibility
type (
	ServerInterface = handler.ServerInterface
	AppInterface    = handler.AppInterface
)
