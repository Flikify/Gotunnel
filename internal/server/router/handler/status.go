package handler

import (
	"github.com/gin-gonic/gin"
	// removed router import
	"github.com/gotunnel/internal/server/router/dto"
)

// StatusHandler 状态处理器
type StatusHandler struct {
	app AppInterface
}

// NewStatusHandler 创建状态处理器
func NewStatusHandler(app AppInterface) *StatusHandler {
	return &StatusHandler{app: app}
}

// GetStatus 获取服务器状态
// @Summary 获取服务器状态
// @Description 返回服务器运行状态和客户端数量
// @Tags 状态
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=dto.StatusResponse}
// @Router /api/status [get]
func (h *StatusHandler) GetStatus(c *gin.Context) {
	clients, _ := h.app.GetClientStore().GetAllClients()

	status := dto.StatusResponse{
		Server: dto.ServerStatus{
			BindAddr: h.app.GetServer().GetBindAddr(),
			BindPort: h.app.GetServer().GetBindPort(),
		},
		ClientCount: len(clients),
	}

	Success(c, status)
}

// GetVersion 获取版本信息
// @Summary 获取版本信息
// @Description 返回服务器版本信息
// @Tags 状态
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=dto.VersionInfo}
// @Router /api/update/version [get]
func (h *StatusHandler) GetVersion(c *gin.Context) {
	Success(c, getVersionInfo())
}
