package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/db"
	// removed router import
	"github.com/gotunnel/internal/server/router/dto"
)

// JSPluginHandler JS 插件处理器
type JSPluginHandler struct {
	app AppInterface
}

// NewJSPluginHandler 创建 JS 插件处理器
func NewJSPluginHandler(app AppInterface) *JSPluginHandler {
	return &JSPluginHandler{app: app}
}

// List 获取 JS 插件列表
// @Summary 获取所有 JS 插件
// @Description 返回所有注册的 JS 插件
// @Tags JS插件
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=[]db.JSPlugin}
// @Router /api/js-plugins [get]
func (h *JSPluginHandler) List(c *gin.Context) {
	plugins, err := h.app.GetJSPluginStore().GetAllJSPlugins()
	if err != nil {
		InternalError(c, err.Error())
		return
	}
	if plugins == nil {
		plugins = []db.JSPlugin{}
	}
	Success(c, plugins)
}

// Create 创建 JS 插件
// @Summary 创建 JS 插件
// @Description 创建新的 JS 插件
// @Tags JS插件
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.JSPluginCreateRequest true "插件信息"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Router /api/js-plugins [post]
func (h *JSPluginHandler) Create(c *gin.Context) {
	var req dto.JSPluginCreateRequest
	if !BindJSON(c, &req) {
		return
	}

	plugin := &db.JSPlugin{
		Name:        req.Name,
		Source:      req.Source,
		Signature:   req.Signature,
		Description: req.Description,
		Author:      req.Author,
		Config:      req.Config,
		AutoStart:   req.AutoStart,
		Enabled:     true,
	}

	if err := h.app.GetJSPluginStore().SaveJSPlugin(plugin); err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{"status": "ok"})
}

// Get 获取单个 JS 插件
// @Summary 获取 JS 插件详情
// @Description 获取指定 JS 插件的详细信息
// @Tags JS插件
// @Produce json
// @Security Bearer
// @Param name path string true "插件名称"
// @Success 200 {object} Response{data=db.JSPlugin}
// @Failure 404 {object} Response
// @Router /api/js-plugin/{name} [get]
func (h *JSPluginHandler) Get(c *gin.Context) {
	name := c.Param("name")

	plugin, err := h.app.GetJSPluginStore().GetJSPlugin(name)
	if err != nil {
		NotFound(c, "plugin not found")
		return
	}

	Success(c, plugin)
}

// Update 更新 JS 插件
// @Summary 更新 JS 插件
// @Description 更新指定 JS 插件的信息
// @Tags JS插件
// @Accept json
// @Produce json
// @Security Bearer
// @Param name path string true "插件名称"
// @Param request body dto.JSPluginUpdateRequest true "更新内容"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Router /api/js-plugin/{name} [put]
func (h *JSPluginHandler) Update(c *gin.Context) {
	name := c.Param("name")

	var req dto.JSPluginUpdateRequest
	if !BindJSON(c, &req) {
		return
	}

	plugin := &db.JSPlugin{
		Name:        name,
		Source:      req.Source,
		Signature:   req.Signature,
		Description: req.Description,
		Author:      req.Author,
		Config:      req.Config,
		AutoStart:   req.AutoStart,
		Enabled:     req.Enabled,
	}

	if err := h.app.GetJSPluginStore().SaveJSPlugin(plugin); err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{"status": "ok"})
}

// Delete 删除 JS 插件
// @Summary 删除 JS 插件
// @Description 删除指定的 JS 插件
// @Tags JS插件
// @Produce json
// @Security Bearer
// @Param name path string true "插件名称"
// @Success 200 {object} Response
// @Router /api/js-plugin/{name} [delete]
func (h *JSPluginHandler) Delete(c *gin.Context) {
	name := c.Param("name")

	if err := h.app.GetJSPluginStore().DeleteJSPlugin(name); err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{"status": "ok"})
}

// PushToClient 推送 JS 插件到客户端
// @Summary 推送插件到客户端
// @Description 将 JS 插件推送到指定客户端
// @Tags JS插件
// @Accept json
// @Produce json
// @Security Bearer
// @Param name path string true "插件名称"
// @Param clientID path string true "客户端ID"
// @Param request body dto.JSPluginPushRequest false "推送配置"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /api/js-plugin/{name}/push/{clientID} [post]
func (h *JSPluginHandler) PushToClient(c *gin.Context) {
	pluginName := c.Param("name")
	clientID := c.Param("clientID")

	// 解析请求体（可选）
	var pushReq dto.JSPluginPushRequest
	c.ShouldBindJSON(&pushReq) // 忽略错误，允许空请求体

	// 检查客户端是否在线
	online, _, _, _, _, _ := h.app.GetServer().GetClientStatus(clientID)
	if !online {
		ClientNotOnline(c)
		return
	}

	// 获取插件
	plugin, err := h.app.GetJSPluginStore().GetJSPlugin(pluginName)
	if err != nil {
		NotFound(c, "plugin not found")
		return
	}

	if !plugin.Enabled {
		Error(c, 400, CodePluginDisabled, "plugin is disabled")
		return
	}

	// 推送到客户端
	req := JSPluginInstallRequest{
		PluginName: plugin.Name,
		Source:     plugin.Source,
		Signature:  plugin.Signature,
		RuleName:   plugin.Name,
		RemotePort: pushReq.RemotePort,
		Config:     plugin.Config,
		AutoStart:  plugin.AutoStart,
	}

	if err := h.app.GetServer().InstallJSPluginToClient(clientID, req); err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{
		"status":      "ok",
		"plugin":      pluginName,
		"client":      clientID,
		"remote_port": pushReq.RemotePort,
	})
}
