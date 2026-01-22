package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/router/dto"
	"github.com/gotunnel/pkg/protocol"
)

// ClientHandler 客户端处理器
type ClientHandler struct {
	app AppInterface
}

// NewClientHandler 创建客户端处理器
func NewClientHandler(app AppInterface) *ClientHandler {
	return &ClientHandler{app: app}
}

// List 获取客户端列表
// @Summary 获取所有客户端
// @Description 返回所有注册客户端的列表及其在线状态
// @Tags 客户端
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=[]dto.ClientListItem}
// @Router /api/clients [get]
func (h *ClientHandler) List(c *gin.Context) {
	clients, err := h.app.GetClientStore().GetAllClients()
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	statusMap := h.app.GetServer().GetAllClientStatus()
	result := make([]dto.ClientListItem, 0, len(clients))

	for _, client := range clients {
		item := dto.ClientListItem{
			ID:        client.ID,
			Nickname:  client.Nickname,
			RuleCount: len(client.Rules),
		}
		if status, ok := statusMap[client.ID]; ok {
			item.Online = status.Online
			item.LastPing = status.LastPing
			item.RemoteAddr = status.RemoteAddr
			item.OS = status.OS
			item.Arch = status.Arch
		}
		result = append(result, item)
	}

	Success(c, result)
}

// Create 创建客户端
// @Summary 创建新客户端
// @Description 创建一个新的客户端配置
// @Tags 客户端
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.CreateClientRequest true "客户端信息"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 409 {object} Response
// @Router /api/clients [post]
func (h *ClientHandler) Create(c *gin.Context) {
	var req dto.CreateClientRequest
	if !BindJSON(c, &req) {
		return
	}

	// 验证客户端 ID 格式
	if !validateClientID(req.ID) {
		BadRequest(c, "invalid client id: must be 1-64 alphanumeric characters, underscore or hyphen")
		return
	}

	// 检查客户端是否已存在
	exists, _ := h.app.GetClientStore().ClientExists(req.ID)
	if exists {
		Conflict(c, "client already exists")
		return
	}

	client := &db.Client{ID: req.ID, Rules: req.Rules}
	if err := h.app.GetClientStore().CreateClient(client); err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{"status": "ok"})
}

// Get 获取单个客户端
// @Summary 获取客户端详情
// @Description 获取指定客户端的详细信息
// @Tags 客户端
// @Produce json
// @Security Bearer
// @Param id path string true "客户端ID"
// @Success 200 {object} Response{data=dto.ClientResponse}
// @Failure 404 {object} Response
// @Router /api/client/{id} [get]
func (h *ClientHandler) Get(c *gin.Context) {
	clientID := c.Param("id")

	client, err := h.app.GetClientStore().GetClient(clientID)
	if err != nil {
		NotFound(c, "client not found")
		return
	}

	online, lastPing, remoteAddr, clientOS, clientArch := h.app.GetServer().GetClientStatus(clientID)

	// 复制插件列表
	plugins := make([]db.ClientPlugin, len(client.Plugins))
	copy(plugins, client.Plugins)

	// 如果客户端在线，获取实时插件运行状态
	if online {
		if statusList, err := h.app.GetServer().GetClientPluginStatus(clientID); err == nil {
			// 创建运行中插件的映射
			runningPlugins := make(map[string]bool)
			for _, s := range statusList {
				runningPlugins[s.PluginName] = s.Running
			}
			// 更新插件状态
			for i := range plugins {
				if running, ok := runningPlugins[plugins[i].Name]; ok {
					plugins[i].Running = running
				} else {
					plugins[i].Running = false
				}
			}
		}
	} else {
		// 客户端离线时，所有插件都标记为未运行
		for i := range plugins {
			plugins[i].Running = false
		}
	}

	resp := dto.ClientResponse{
		ID:         client.ID,
		Nickname:   client.Nickname,
		Rules:      client.Rules,
		Plugins:    plugins,
		Online:     online,
		LastPing:   lastPing,
		RemoteAddr: remoteAddr,
		OS:         clientOS,
		Arch:       clientArch,
	}

	Success(c, resp)
}

// Update 更新客户端
// @Summary 更新客户端配置
// @Description 更新指定客户端的配置信息
// @Tags 客户端
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "客户端ID"
// @Param request body dto.UpdateClientRequest true "更新内容"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /api/client/{id} [put]
func (h *ClientHandler) Update(c *gin.Context) {
	clientID := c.Param("id")

	var req dto.UpdateClientRequest
	if !BindJSON(c, &req) {
		return
	}

	client, err := h.app.GetClientStore().GetClient(clientID)
	if err != nil {
		NotFound(c, "client not found")
		return
	}

	client.Nickname = req.Nickname
	client.Rules = req.Rules
	if req.Plugins != nil {
		client.Plugins = req.Plugins
	}

	if err := h.app.GetClientStore().UpdateClient(client); err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{"status": "ok"})
}

// Delete 删除客户端
// @Summary 删除客户端
// @Description 删除指定的客户端配置
// @Tags 客户端
// @Produce json
// @Security Bearer
// @Param id path string true "客户端ID"
// @Success 200 {object} Response
// @Failure 404 {object} Response
// @Router /api/client/{id} [delete]
func (h *ClientHandler) Delete(c *gin.Context) {
	clientID := c.Param("id")

	exists, _ := h.app.GetClientStore().ClientExists(clientID)
	if !exists {
		NotFound(c, "client not found")
		return
	}

	if err := h.app.GetClientStore().DeleteClient(clientID); err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{"status": "ok"})
}

// PushConfig 推送配置到客户端
// @Summary 推送配置
// @Description 将配置推送到在线客户端
// @Tags 客户端
// @Produce json
// @Security Bearer
// @Param id path string true "客户端ID"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Router /api/client/{id}/push [post]
func (h *ClientHandler) PushConfig(c *gin.Context) {
	clientID := c.Param("id")

	online, _, _, _, _ := h.app.GetServer().GetClientStatus(clientID)
	if !online {
		ClientNotOnline(c)
		return
	}

	if err := h.app.GetServer().PushConfigToClient(clientID); err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{"status": "ok"})
}

// Disconnect 断开客户端连接
// @Summary 断开连接
// @Description 强制断开客户端连接
// @Tags 客户端
// @Produce json
// @Security Bearer
// @Param id path string true "客户端ID"
// @Success 200 {object} Response
// @Router /api/client/{id}/disconnect [post]
func (h *ClientHandler) Disconnect(c *gin.Context) {
	clientID := c.Param("id")

	if err := h.app.GetServer().DisconnectClient(clientID); err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{"status": "ok"})
}

// Restart 重启客户端
// @Summary 重启客户端
// @Description 发送重启命令到客户端
// @Tags 客户端
// @Produce json
// @Security Bearer
// @Param id path string true "客户端ID"
// @Success 200 {object} Response
// @Router /api/client/{id}/restart [post]
func (h *ClientHandler) Restart(c *gin.Context) {
	clientID := c.Param("id")

	if err := h.app.GetServer().RestartClient(clientID); err != nil {
		InternalError(c, err.Error())
		return
	}

	SuccessWithMessage(c, gin.H{"status": "ok"}, "client restart initiated")
}

// InstallPlugins 安装插件到客户端
// @Summary 安装插件
// @Description 将指定插件安装到客户端
// @Tags 客户端
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "客户端ID"
// @Param request body dto.InstallPluginsRequest true "插件列表"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Router /api/client/{id}/install-plugins [post]
func (h *ClientHandler) InstallPlugins(c *gin.Context) {
	clientID := c.Param("id")

	online, _, _, _, _ := h.app.GetServer().GetClientStatus(clientID)
	if !online {
		ClientNotOnline(c)
		return
	}

	var req dto.InstallPluginsRequest
	if !BindJSON(c, &req) {
		return
	}

	if err := h.app.GetServer().InstallPluginsToClient(clientID, req.Plugins); err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{"status": "ok"})
}

// PluginAction 客户端插件操作
// @Summary 插件操作
// @Description 对客户端插件执行操作(start/stop/restart/config/delete)
// @Tags 客户端
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "客户端ID"
// @Param pluginID path string true "插件实例ID"
// @Param action path string true "操作类型" Enums(start, stop, restart, config, delete)
// @Param request body dto.ClientPluginActionRequest false "操作参数"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Router /api/client/{id}/plugin/{pluginID}/{action} [post]
func (h *ClientHandler) PluginAction(c *gin.Context) {
	clientID := c.Param("id")
	pluginID := c.Param("pluginID")
	action := c.Param("action")

	var req dto.ClientPluginActionRequest
	c.ShouldBindJSON(&req) // 忽略错误，使用默认值

	// 通过 pluginID 查找插件信息
	client, err := h.app.GetClientStore().GetClient(clientID)
	if err != nil {
		NotFound(c, "client not found")
		return
	}

	var pluginName string
	for _, p := range client.Plugins {
		if p.ID == pluginID {
			pluginName = p.Name
			break
		}
	}
	if pluginName == "" {
		NotFound(c, "plugin not found")
		return
	}

	if req.RuleName == "" {
		req.RuleName = pluginName
	}

	switch action {
	case "start":
		err = h.app.GetServer().StartClientPlugin(clientID, pluginID, pluginName, req.RuleName)
	case "stop":
		err = h.app.GetServer().StopClientPlugin(clientID, pluginID, pluginName, req.RuleName)
	case "restart":
		err = h.app.GetServer().RestartClientPlugin(clientID, pluginID, pluginName, req.RuleName)
	case "config":
		if req.Config == nil {
			BadRequest(c, "config required")
			return
		}
		err = h.app.GetServer().UpdateClientPluginConfig(clientID, pluginID, pluginName, req.RuleName, req.Config, req.Restart)
	case "delete":
		err = h.deleteClientPlugin(clientID, pluginID)
	default:
		BadRequest(c, "unknown action: "+action)
		return
	}

	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{
		"status":    "ok",
		"action":    action,
		"plugin_id": pluginID,
		"plugin":    pluginName,
	})
}

func (h *ClientHandler) deleteClientPlugin(clientID, pluginID string) error {
	client, err := h.app.GetClientStore().GetClient(clientID)
	if err != nil {
		return fmt.Errorf("client not found")
	}

	var newPlugins []db.ClientPlugin
	var pluginName string
	var pluginPort int
	found := false
	for _, p := range client.Plugins {
		if p.ID == pluginID {
			found = true
			pluginName = p.Name
			pluginPort = p.RemotePort
			continue
		}
		newPlugins = append(newPlugins, p)
	}

	if !found {
		return fmt.Errorf("plugin %s not found", pluginID)
	}

	// 删除插件管理的代理规则
	var newRules []protocol.ProxyRule
	for _, r := range client.Rules {
		if r.PluginManaged && r.Name == pluginName {
			continue // 跳过此插件的规则
		}
		newRules = append(newRules, r)
	}

	// 停止端口监听器
	if pluginPort > 0 {
		h.app.GetServer().StopPluginRule(clientID, pluginPort)
	}

	client.Plugins = newPlugins
	client.Rules = newRules
	return h.app.GetClientStore().UpdateClient(client)
}

// validateClientID 验证客户端 ID 格式
func validateClientID(id string) bool {
	if len(id) < 1 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
