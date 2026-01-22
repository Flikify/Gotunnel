package handler

import (
	"github.com/gin-gonic/gin"
	// removed router import
	"github.com/gotunnel/internal/server/router/dto"
	"github.com/gotunnel/pkg/version"
)

// UpdateHandler 更新处理器
type UpdateHandler struct {
	app AppInterface
}

// NewUpdateHandler 创建更新处理器
func NewUpdateHandler(app AppInterface) *UpdateHandler {
	return &UpdateHandler{app: app}
}

// CheckServer 检查服务端更新
// @Summary 检查服务端更新
// @Description 检查是否有新的服务端版本可用
// @Tags 更新
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=dto.CheckUpdateResponse}
// @Router /api/update/check/server [get]
func (h *UpdateHandler) CheckServer(c *gin.Context) {
	updateInfo, err := checkUpdateForComponent("server")
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	Success(c, updateInfo)
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
// @Router /api/update/check/client [get]
func (h *UpdateHandler) CheckClient(c *gin.Context) {
	var query dto.CheckClientUpdateQuery
	if !BindQuery(c, &query) {
		return
	}

	updateInfo, err := checkClientUpdateForPlatform(query.OS, query.Arch)
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
// @Router /api/update/apply/server [post]
func (h *UpdateHandler) ApplyServer(c *gin.Context) {
	var req dto.ApplyServerUpdateRequest
	if !BindJSON(c, &req) {
		return
	}

	// 异步执行更新
	go func() {
		if err := performSelfUpdate(req.DownloadURL, req.Restart); err != nil {
			println("[Update] Server update failed:", err.Error())
		}
	}()

	Success(c, gin.H{
		"success": true,
		"message": "Update started, server will restart shortly",
	})
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
// @Router /api/update/apply/client [post]
func (h *UpdateHandler) ApplyClient(c *gin.Context) {
	var req dto.ApplyClientUpdateRequest
	if !BindJSON(c, &req) {
		return
	}

	// 发送更新命令到客户端
	if err := h.app.GetServer().SendUpdateToClient(req.ClientID, req.DownloadURL); err != nil {
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
