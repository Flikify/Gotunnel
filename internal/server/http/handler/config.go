package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/http/dto"
	"github.com/gotunnel/internal/server/service"
)

// ConfigHandler 配置处理器
type ConfigHandler struct {
	config service.ConfigService
}

// NewConfigHandler 创建配置处理器
func NewConfigHandler(config service.ConfigService) *ConfigHandler {
	return &ConfigHandler{config: config}
}

// Get 获取服务器配置
// @Summary 获取配置
// @Description 返回服务器配置（敏感信息脱敏）
// @Tags 配置
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=dto.ServerConfigResponse}
// @Router /api/runtime/config [get]
func (h *ConfigHandler) Get(c *gin.Context) {
	cfg := h.config.Snapshot()

	// Token 脱敏处理，只显示前4位
	maskedToken := cfg.Server.Token
	if len(maskedToken) > 4 {
		maskedToken = maskedToken[:4] + "****"
	}

	resp := dto.ServerConfigResponse{
		Server: dto.ServerConfigInfo{
			BindAddr:                 cfg.Server.BindAddr,
			BindPort:                 cfg.Server.BindPort,
			Token:                    maskedToken,
			HeartbeatSec:             cfg.Server.HeartbeatSec,
			HeartbeatTimeout:         cfg.Server.HeartbeatTimeout,
			MaxClientProxies:         cfg.Server.MaxClientProxies,
			ClientResponseTimeoutSec: cfg.Server.ClientResponseTimeoutSec,
		},
		Web: dto.WebConfigInfo{
			Enabled:   cfg.Server.Web.Enabled,
			BindPort:  cfg.Server.Web.BindPort,
			Username:  cfg.Server.Web.Username,
			Password:  "****",
			CDNPrefix: cfg.Server.Web.CDNPrefix,
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
// @Success 200 {object} Response{data=dto.ConfigUpdateResponse}
// @Failure 400 {object} Response
// @Router /api/runtime/config [put]
func (h *ConfigHandler) Update(c *gin.Context) {
	var req dto.UpdateServerConfigRequest
	if !BindJSON(c, &req) {
		return
	}

	persisted, err := h.config.Persist(req.ToConfigUpdate())
	if err != nil {
		if errors.Is(err, service.ErrInvalidHeartbeatConfig) {
			BadRequest(c, err.Error())
			return
		}
		InternalError(c, err.Error())
		return
	}

	result := h.config.ApplyRuntimeConfig(persisted.RuntimeApplyFields)
	result.RestartRequiredFields = persisted.RestartRequiredFields

	message := "配置已保存并同步了可热生效项"
	if len(result.RestartRequiredFields) > 0 {
		message = "配置已保存，部分变更需要重启后生效"
	}

	SuccessWithMessage(c, dto.ConfigUpdateResponse{
		Status:                "ok",
		AppliedRuntimeFields:  result.AppliedRuntimeFields,
		RestartRequiredFields: result.RestartRequiredFields,
	}, message)
}
