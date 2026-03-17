package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/router/dto"
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

	online, lastPing, remoteAddr, clientName, clientOS, clientArch, clientVersion := h.app.GetServer().GetClientStatus(clientID)

	// 如果客户端在线且有名称，优先使用在线名称
	nickname := client.Nickname
	if online && clientName != "" && nickname == "" {
		nickname = clientName
	}

	resp := dto.ClientResponse{
		ID:         client.ID,
		Nickname:   nickname,
		Rules:      client.Rules,
		Online:     online,
		LastPing:   lastPing,
		RemoteAddr: remoteAddr,
		OS:         clientOS,
		Arch:       clientArch,
		Version:    clientVersion,
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

	if !h.app.GetServer().IsClientOnline(clientID) {
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

// @Failure 400 {object} Response
// @Router /api/client/{id}/install-plugins [post]

// @Failure 400 {object} Response
// @Router /api/client/{id}/plugin/{pluginID}/{action} [post]


// GetSystemStats 获取客户端系统状态
func (h *ClientHandler) GetSystemStats(c *gin.Context) {
	clientID := c.Param("id")
	stats, err := h.app.GetServer().GetClientSystemStats(clientID)
	if err != nil {
		ClientNotOnline(c)
		return
	}
	Success(c, stats)
}

// GetScreenshot 获取客户端截图
func (h *ClientHandler) GetScreenshot(c *gin.Context) {
	clientID := c.Param("id")
	quality := 0
	if q, ok := c.GetQuery("quality"); ok {
		fmt.Sscanf(q, "%d", &quality)
	}

	screenshot, err := h.app.GetServer().GetClientScreenshot(clientID, quality)
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, screenshot)
}

// ExecuteShellRequest Shell 执行请求体
type ExecuteShellRequest struct {
	Command string `json:"command" binding:"required"`
	Timeout int    `json:"timeout"`
}

// ExecuteShell 执行 Shell 命令
func (h *ClientHandler) ExecuteShell(c *gin.Context) {
	clientID := c.Param("id")
	var req ExecuteShellRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.app.GetServer().ExecuteClientShell(clientID, req.Command, req.Timeout)
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, result)
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
