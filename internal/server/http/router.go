package httpapi

import (
	"io"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/service"
	db "github.com/gotunnel/internal/server/storage/sqlite"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/gotunnel/internal/server/http/handler"
	"github.com/gotunnel/internal/server/http/middleware"
	"github.com/gotunnel/pkg/auth"
)

// Dependencies declares the explicit contracts required to assemble HTTP routes.
type Dependencies struct {
	ClientStore       db.ClientStore
	InstallTokenStore db.InstallTokenStore
	ServerRuntime     handler.ServerInterface
	ConfigService     service.ConfigService
	TrafficStore      db.TrafficStore
	OperationalEvents db.OperationalEventStore
	JWTAuth           *auth.JWTAuth
	Username          string
	Password          string
}

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
func (r *GinRouter) SetupRoutes(deps Dependencies) {
	engine := r.Engine
	remoteOps := service.NewRemoteOpsService(deps.ServerRuntime)
	remoteControl := service.NewRemoteControlService(deps.ServerRuntime)
	diagnostics := service.NewDiagnosticsService(deps.ServerRuntime)
	events := service.NewEventService(deps.OperationalEvents)

	// 全局中间件
	engine.Use(middleware.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())

	// Swagger 文档
	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 认证路由 (无需 JWT)
	authHandler := handler.NewAuthHandler(deps.Username, deps.Password, deps.JWTAuth)
	engine.POST("/api/auth/login", authHandler.Login)
	engine.GET("/api/auth/check", authHandler.Check)

	installHandler := handler.NewInstallHandler(deps.InstallTokenStore, deps.ServerRuntime)
	engine.GET("/install.sh", installHandler.ServeBashInstallScript)
	engine.GET("/install.ps1", installHandler.ServePowerShellScript)

	// API 路由 (需要 JWT)
	api := engine.Group("/api")
	api.Use(middleware.JWTAuth(deps.JWTAuth))
	{
		statusHandler := handler.NewStatusHandler(deps.ClientStore, deps.ServerRuntime)
		api.GET("/runtime/status", statusHandler.GetStatus)
		api.GET("/runtime/version", statusHandler.GetVersion)

		clientService := service.NewClientService(deps.ClientStore, service.NewClientRuntimeAdapter(deps.ServerRuntime), deps.ConfigService)
		clientHandler := handler.NewClientHandler(clientService, remoteOps)
		remoteControlHandler := handler.NewRemoteControlHandler(remoteControl)
		api.GET("/clients", clientHandler.List)
		api.POST("/clients", clientHandler.Create)
		api.GET("/clients/:id", clientHandler.Get)
		api.PUT("/clients/:id", clientHandler.Update)
		api.DELETE("/clients/:id", clientHandler.Delete)
		api.POST("/clients/:id/actions/push-config", clientHandler.PushConfig)
		api.POST("/clients/:id/actions/disconnect", clientHandler.Disconnect)
		api.POST("/clients/:id/actions/restart", clientHandler.Restart)
		api.GET("/clients/:id/system-stats", clientHandler.GetSystemStats)
		api.GET("/clients/:id/screenshot", clientHandler.GetScreenshot)
		api.GET("/clients/:id/remote-control/ws", remoteControlHandler.Stream)

		configHandler := handler.NewConfigHandler(deps.ConfigService)
		api.GET("/runtime/config", configHandler.Get)
		api.PUT("/runtime/config", configHandler.Update)

		updateHandler := handler.NewUpdateHandler(service.NewUpdateService(deps.ServerRuntime, deps.ConfigService))
		api.GET("/updates/server", updateHandler.CheckServer)
		api.GET("/updates/server/status", updateHandler.CheckServerStatus)
		api.GET("/updates/clients/latest", updateHandler.CheckClient)
		api.POST("/updates/server/actions/apply", updateHandler.ApplyServer)
		api.POST("/updates/clients/actions/apply", updateHandler.ApplyClient)

		logHandler := handler.NewLogHandler(diagnostics)
		api.GET("/clients/:id/logs", logHandler.StreamLogs)

		obsHandler := handler.NewObservabilityHandler(events, diagnostics)
		api.GET("/events", obsHandler.ListEvents)
		api.GET("/events/health", obsHandler.Health)
		api.POST("/nodes/:id/diagnostics/query", obsHandler.QueryDiagnostics)
		api.GET("/nodes/:id/diagnostics/stream", obsHandler.StreamDiagnostics)

		trafficHandler := handler.NewTrafficHandler(deps.TrafficStore)
		api.GET("/runtime/traffic/stats", trafficHandler.GetStats)
		api.GET("/runtime/traffic/hourly", trafficHandler.GetHourly)

		api.POST("/installations/actions/command", installHandler.GenerateInstallCommand)
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
type ServerInterface = handler.ServerInterface
