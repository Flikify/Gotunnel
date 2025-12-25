package router

import (
	"net/http"
)

// Router 路由管理器
type Router struct {
	mux *http.ServeMux
}

// New 创建路由管理器
func New() *Router {
	return &Router{
		mux: http.NewServeMux(),
	}
}

// Handle 注册路由处理器
func (r *Router) Handle(pattern string, handler http.Handler) {
	r.mux.Handle(pattern, handler)
}

// HandleFunc 注册路由处理函数
func (r *Router) HandleFunc(pattern string, handler http.HandlerFunc) {
	r.mux.HandleFunc(pattern, handler)
}

// Group 创建路由组
func (r *Router) Group(prefix string) *RouteGroup {
	return &RouteGroup{
		router: r,
		prefix: prefix,
	}
}

// RouteGroup 路由组
type RouteGroup struct {
	router *Router
	prefix string
}

// HandleFunc 注册路由组处理函数
func (g *RouteGroup) HandleFunc(pattern string, handler http.HandlerFunc) {
	g.router.mux.HandleFunc(g.prefix+pattern, handler)
}

// Handler 返回 http.Handler
func (r *Router) Handler() http.Handler {
	return r.mux
}
