package handler

import (
	"github.com/gin-gonic/gin"
	// removed router import
	"github.com/gotunnel/internal/server/router/dto"
)

// ConfigHandler 配置处理器
type ConfigHandler struct {
	app AppInterface
}

// NewConfigHandler 创建配置处理器
func NewConfigHandler(app AppInterface) *ConfigHandler {
	return &ConfigHandler{app: app}
}

// Get 获取服务器配置
// @Summary 获取配置
// @Description 返回服务器配置（敏感信息脱敏）
// @Tags 配置
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=dto.ServerConfigResponse}
// @Router /api/config [get]
func (h *ConfigHandler) Get(c *gin.Context) {
	cfg := h.app.GetConfig()

	// Token 脱敏处理，只显示前4位
	maskedToken := cfg.Server.Token
	if len(maskedToken) > 4 {
		maskedToken = maskedToken[:4] + "****"
	}

	resp := dto.ServerConfigResponse{
		Server: dto.ServerConfigInfo{
			BindAddr:         cfg.Server.BindAddr,
			BindPort:         cfg.Server.BindPort,
			Token:            maskedToken,
			HeartbeatSec:     cfg.Server.HeartbeatSec,
			HeartbeatTimeout: cfg.Server.HeartbeatTimeout,
		},
		Web: dto.WebConfigInfo{
			Enabled:  cfg.Server.Web.Enabled,
			BindPort: cfg.Server.Web.BindPort,
			Username: cfg.Server.Web.Username,
			Password: "****",
		},
		PluginStore: dto.PluginStoreConfigInfo{
			URL: cfg.Server.PluginStore.URL,
		},
	}

	Success(c, resp)
}

// Update 更新服务器配置
// @Summary 更新配置
// @Description 更新服务器配置
// @Tags 配置
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.UpdateServerConfigRequest true "配置内容"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Router /api/config [put]
func (h *ConfigHandler) Update(c *gin.Context) {
	var req dto.UpdateServerConfigRequest
	if !BindJSON(c, &req) {
		return
	}

	cfg := h.app.GetConfig()

	// 更新 Server 配置
	if req.Server != nil {
		if req.Server.BindAddr != "" {
			cfg.Server.BindAddr = req.Server.BindAddr
		}
		if req.Server.BindPort > 0 {
			cfg.Server.BindPort = req.Server.BindPort
		}
		if req.Server.Token != "" {
			cfg.Server.Token = req.Server.Token
		}
		if req.Server.HeartbeatSec > 0 {
			cfg.Server.HeartbeatSec = req.Server.HeartbeatSec
		}
		if req.Server.HeartbeatTimeout > 0 {
			cfg.Server.HeartbeatTimeout = req.Server.HeartbeatTimeout
		}
	}

	// 更新 Web 配置
	if req.Web != nil {
		cfg.Server.Web.Enabled = req.Web.Enabled
		if req.Web.BindPort > 0 {
			cfg.Server.Web.BindPort = req.Web.BindPort
		}
		cfg.Server.Web.Username = req.Web.Username
		cfg.Server.Web.Password = req.Web.Password
	}

	// 更新 PluginStore 配置
	if req.PluginStore != nil {
		cfg.Server.PluginStore.URL = req.PluginStore.URL
	}

	if err := h.app.SaveConfig(); err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{"status": "ok"})
}

// Reload 重新加载配置
// @Summary 重新加载配置
// @Description 重新加载服务器配置
// @Tags 配置
// @Produce json
// @Security Bearer
// @Success 200 {object} Response
// @Failure 500 {object} Response
// @Router /api/config/reload [post]
func (h *ConfigHandler) Reload(c *gin.Context) {
	if err := h.app.GetServer().ReloadConfig(); err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{"status": "ok"})
}
