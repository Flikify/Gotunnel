package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/http/dto"
	db "github.com/gotunnel/internal/server/storage/sqlite"
)

// StatusHandler 状态处理器
type StatusHandler struct {
	clientStore db.ClientStore
	serverInfo  ServerInfoInterface
}

// NewStatusHandler 创建状态处理器
func NewStatusHandler(clientStore db.ClientStore, serverInfo ServerInfoInterface) *StatusHandler {
	return &StatusHandler{
		clientStore: clientStore,
		serverInfo:  serverInfo,
	}
}

// GetStatus 获取服务器状态
// @Summary 获取服务器状态
// @Description 返回服务器运行状态和客户端数量
// @Tags 状态
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=dto.StatusResponse}
// @Router /api/runtime/status [get]
func (h *StatusHandler) GetStatus(c *gin.Context) {
	clients, _ := h.clientStore.GetAllClients()

	status := dto.StatusResponse{
		Server: dto.ServerStatus{
			BindAddr: h.serverInfo.GetBindAddr(),
			BindPort: h.serverInfo.GetBindPort(),
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
// @Router /api/runtime/version [get]
func (h *StatusHandler) GetVersion(c *gin.Context) {
	Success(c, getVersionInfo())
}
