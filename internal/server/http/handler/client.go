package handler

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	domain "github.com/gotunnel/internal/core/domain"
	"github.com/gotunnel/internal/server/http/dto"
	"github.com/gotunnel/internal/server/service"
)

// ClientHandler 客户端处理器
type ClientHandler struct {
	clients   service.ClientService
	remoteOps service.RemoteOpsService
}

// NewClientHandler 创建客户端处理器
func NewClientHandler(clients service.ClientService, remoteOps service.RemoteOpsService) *ClientHandler {
	return &ClientHandler{
		clients:   clients,
		remoteOps: remoteOps,
	}
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
	clients, err := h.clients.ListClients()
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, toClientListItems(clients))
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

	if err := h.clients.CreateClient(service.CreateClientInput{
		ID:    req.ID,
		Rules: toDomainRules(req.Rules),
	}); err != nil {
		h.handleClientServiceError(c, err)
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
// @Router /api/clients/{id} [get]
func (h *ClientHandler) Get(c *gin.Context) {
	clientID := c.Param("id")

	client, err := h.clients.GetClient(clientID)
	if err != nil {
		h.handleClientServiceError(c, err)
		return
	}

	Success(c, toClientResponse(client))
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
// @Router /api/clients/{id} [put]
func (h *ClientHandler) Update(c *gin.Context) {
	clientID := c.Param("id")

	var req dto.UpdateClientRequest
	if !BindJSON(c, &req) {
		return
	}

	if err := h.clients.UpdateClient(clientID, service.UpdateClientInput{
		Nickname: req.Nickname,
		Rules:    toDomainRules(req.Rules),
	}); err != nil {
		h.handleClientServiceError(c, err)
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
// @Router /api/clients/{id} [delete]
func (h *ClientHandler) Delete(c *gin.Context) {
	clientID := c.Param("id")

	if err := h.clients.DeleteClient(clientID); err != nil {
		h.handleClientServiceError(c, err)
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
// @Router /api/clients/{id}/actions/push-config [post]
func (h *ClientHandler) PushConfig(c *gin.Context) {
	clientID := c.Param("id")

	if err := h.clients.PushConfig(clientID); err != nil {
		h.handleClientServiceError(c, err)
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
// @Router /api/clients/{id}/actions/disconnect [post]
func (h *ClientHandler) Disconnect(c *gin.Context) {
	clientID := c.Param("id")

	if err := h.clients.DisconnectClient(clientID); err != nil {
		h.handleClientServiceError(c, err)
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
// @Router /api/clients/{id}/actions/restart [post]
func (h *ClientHandler) Restart(c *gin.Context) {
	clientID := c.Param("id")

	if err := h.clients.RestartClient(clientID); err != nil {
		h.handleClientServiceError(c, err)
		return
	}

	SuccessWithMessage(c, gin.H{"status": "ok"}, "client restart initiated")
}

// GetSystemStats 获取客户端系统状态
// @Summary 获取客户端系统状态
// @Description 获取在线客户端的系统资源使用情况
// @Tags 客户端
// @Produce json
// @Security Bearer
// @Param id path string true "客户端ID"
// @Success 200 {object} Response{data=dto.SystemStatsResponse}
// @Failure 400 {object} Response
// @Router /api/clients/{id}/system-stats [get]
func (h *ClientHandler) GetSystemStats(c *gin.Context) {
	clientID := c.Param("id")
	stats, err := h.remoteOps.GetClientSystemStats(clientID)
	if err != nil {
		ClientNotOnline(c)
		return
	}
	Success(c, stats)
}

// GetScreenshot 获取客户端截图
// @Summary 获取客户端截图
// @Description 获取在线客户端当前屏幕截图
// @Tags 客户端
// @Produce json
// @Security Bearer
// @Param id path string true "客户端ID"
// @Param quality query int false "JPEG 质量，1-100，0 使用默认值"
// @Success 200 {object} Response{data=dto.ScreenshotResponse}
// @Failure 400 {object} Response
// @Router /api/clients/{id}/screenshot [get]
func (h *ClientHandler) GetScreenshot(c *gin.Context) {
	clientID := c.Param("id")
	quality := 0
	if q, ok := c.GetQuery("quality"); ok {
		fmt.Sscanf(q, "%d", &quality)
	}

	screenshot, err := h.remoteOps.GetClientScreenshot(clientID, quality)
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, screenshot)
}

// ExecuteShell 执行 Shell 命令
// @Summary 执行客户端 Shell 命令
// @Description 在在线客户端执行单条 Shell 命令并返回输出
// @Tags 客户端
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "客户端ID"
// @Param request body dto.ExecuteShellRequest true "Shell 执行参数"
// @Success 200 {object} Response{data=dto.ShellExecuteResponse}
// @Failure 400 {object} Response
// @Router /api/clients/{id}/actions/shell [post]
func (h *ClientHandler) ExecuteShell(c *gin.Context) {
	clientID := c.Param("id")
	var req dto.ExecuteShellRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.remoteOps.ExecuteClientShell(clientID, req.Command, req.Timeout)
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, result)
}

func (h *ClientHandler) handleClientServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidClientID), errors.Is(err, service.ErrProxyRuleLimitExceeded):
		BadRequest(c, err.Error())
	case errors.Is(err, service.ErrClientAlreadyExists):
		Conflict(c, err.Error())
	case errors.Is(err, service.ErrClientNotFound):
		NotFound(c, err.Error())
	case errors.Is(err, service.ErrClientNotOnline):
		ClientNotOnline(c)
	default:
		InternalError(c, err.Error())
	}
}

func toClientListItems(items []service.ClientListItem) []dto.ClientListItem {
	result := make([]dto.ClientListItem, 0, len(items))
	for _, item := range items {
		result = append(result, dto.ClientListItem{
			ID:            item.ID,
			Nickname:      item.Nickname,
			Online:        item.Online,
			LastPing:      item.LastPing,
			LastOfflineAt: item.LastOfflineAt,
			RemoteAddr:    item.RemoteAddr,
			RuleCount:     item.RuleCount,
			OS:            item.OS,
			Arch:          item.Arch,
			Version:       item.Version,
		})
	}
	return result
}

func toClientResponse(client *service.ClientDetail) dto.ClientResponse {
	return dto.ClientResponse{
		ID:            client.ID,
		Nickname:      client.Nickname,
		Rules:         toDTOProxyRules(client.Rules),
		Online:        client.Online,
		LastPing:      client.LastPing,
		LastOfflineAt: client.LastOfflineAt,
		RemoteAddr:    client.RemoteAddr,
		OS:            client.OS,
		Arch:          client.Arch,
		Version:       client.Version,
	}
}

func toDomainRules(rules []dto.ProxyRule) []domain.ProxyRule {
	if len(rules) == 0 {
		return nil
	}
	result := make([]domain.ProxyRule, 0, len(rules))
	for _, rule := range rules {
		result = append(result, domain.ProxyRule{
			Name:         rule.Name,
			Type:         rule.Type,
			LocalIP:      rule.LocalIP,
			LocalPort:    rule.LocalPort,
			RemotePort:   rule.RemotePort,
			Enabled:      rule.Enabled,
			AuthEnabled:  rule.AuthEnabled,
			AuthUsername: rule.AuthUsername,
			AuthPassword: rule.AuthPassword,
			PortStatus:   rule.PortStatus,
		})
	}
	return result
}

func toDTOProxyRules(rules []domain.ProxyRule) []dto.ProxyRule {
	if len(rules) == 0 {
		return nil
	}
	result := make([]dto.ProxyRule, 0, len(rules))
	for _, rule := range rules {
		result = append(result, dto.ProxyRule{
			Name:         rule.Name,
			Type:         rule.Type,
			LocalIP:      rule.LocalIP,
			LocalPort:    rule.LocalPort,
			RemotePort:   rule.RemotePort,
			Enabled:      rule.Enabled,
			AuthEnabled:  rule.AuthEnabled,
			AuthUsername: rule.AuthUsername,
			AuthPassword: rule.AuthPassword,
			PortStatus:   rule.PortStatus,
		})
	}
	return result
}
