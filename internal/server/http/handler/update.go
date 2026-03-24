package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/http/dto"
	"github.com/gotunnel/internal/server/service"
	"github.com/gotunnel/internal/server/updateapp"
	"github.com/gotunnel/pkg/version"
)

// UpdateHandler 更新处理器
type UpdateHandler struct {
	updates service.UpdateService
}

// NewUpdateHandler 创建更新处理器
func NewUpdateHandler(updates service.UpdateService) *UpdateHandler {
	return &UpdateHandler{updates: updates}
}

// CheckServer 检查服务端更新
// @Summary 检查服务端更新
// @Description 检查是否有新的服务端版本可用
// @Tags 更新
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=dto.CheckUpdateResponse}
// @Router /api/updates/server [get]
func (h *UpdateHandler) CheckServer(c *gin.Context) {
	updateInfo, err := h.updates.CheckServer()
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, updateInfo)
}

// CheckServerStatus 查询服务端更新任务状态
// @Summary 获取服务端更新状态
// @Description 返回服务端自更新任务的当前状态
// @Tags 更新
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=dto.ServerUpdateStatusResponse}
// @Router /api/updates/server/status [get]
func (h *UpdateHandler) CheckServerStatus(c *gin.Context) {
	status, err := h.updates.GetServerUpdateStatus()
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, dto.ServerUpdateStatusResponse{
		State:          status.State,
		Message:        status.Message,
		CurrentVersion: status.CurrentVersion,
		TargetVersion:  status.TargetVersion,
		StartedAt:      status.StartedAt,
		FinishedAt:     status.FinishedAt,
		UpdatedAt:      status.UpdatedAt,
	})
}

// CheckClient 检查客户端更新
// @Summary 检查客户端更新
// @Description 检查是否有新的客户端版本可用
// @Tags 更新
// @Produce json
// @Security Bearer
// @Param os query string false "操作系统" Enums(linux, darwin, windows)
// @Param arch query string false "架构" Enums(amd64, arm64, 386, arm)
// @Success 200 {object} Response{data=dto.CheckUpdateResponse}
// @Router /api/updates/clients/latest [get]
func (h *UpdateHandler) CheckClient(c *gin.Context) {
	var query dto.CheckClientUpdateQuery
	if !BindQuery(c, &query) {
		return
	}

	updateInfo, err := h.updates.CheckClient(query.OS, query.Arch)
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, updateInfo)
}

// ApplyServer 应用服务端更新
// @Summary 应用服务端更新
// @Description 下载并应用服务端更新，服务器将自动重启
// @Tags 更新
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.ApplyServerUpdateRequest true "更新请求"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Router /api/updates/server/actions/apply [post]
func (h *UpdateHandler) ApplyServer(c *gin.Context) {
	var req dto.ApplyServerUpdateRequest
	if !BindJSON(c, &req) {
		return
	}

	if err := h.updates.ApplyServer(req.DownloadURL, req.TargetVersion, req.Restart); err != nil {
		if errors.Is(err, updateapp.ErrUpdateInProgress) {
			Conflict(c, "server update already in progress")
			return
		}
		InternalError(c, err.Error())
		return
	}

	SuccessWithMessage(c, gin.H{"status": "ok"}, "Update started, server will restart shortly")
}

// ApplyClient 应用客户端更新
// @Summary 推送客户端更新
// @Description 向指定客户端推送更新命令
// @Tags 更新
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.ApplyClientUpdateRequest true "更新请求"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Router /api/updates/clients/actions/apply [post]
func (h *UpdateHandler) ApplyClient(c *gin.Context) {
	var req dto.ApplyClientUpdateRequest
	if !BindJSON(c, &req) {
		return
	}

	// 发送更新命令到客户端
	if err := h.updates.ApplyClient(req.ClientID, req.DownloadURL); err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, gin.H{
		"success": true,
		"message": "Update command sent to client",
	})
}

// getVersionInfo 获取版本信息
func getVersionInfo() dto.VersionInfo {
	info := version.GetInfo()
	return dto.VersionInfo{
		Version:   info.Version,
		GitCommit: info.GitCommit,
		BuildTime: info.BuildTime,
		GoVersion: info.GoVersion,
		OS:        info.OS,
		Arch:      info.Arch,
	}
}
