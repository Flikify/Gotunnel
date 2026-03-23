package handler

import (
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gotunnel/internal/server/service"
)

// LogHandler 日志处理器
type LogHandler struct {
	remoteOps service.RemoteOpsService
}

// NewLogHandler 创建日志处理器
func NewLogHandler(remoteOps service.RemoteOpsService) *LogHandler {
	return &LogHandler{remoteOps: remoteOps}
}

// StreamLogs 流式传输客户端日志
// @Summary 流式传输客户端日志
// @Description 通过 Server-Sent Events 实时接收客户端日志
// @Tags Logs
// @Produce text/event-stream
// @Security Bearer
// @Param id path string true "客户端 ID"
// @Param lines query int false "初始日志行数" default(100)
// @Param follow query bool false "是否持续推送新日志" default(true)
// @Param level query string false "日志级别过滤 (info, warn, error)"
// @Success 200 {string} string "Server-Sent Events stream"
// @Router /api/clients/{id}/logs [get]
func (h *LogHandler) StreamLogs(c *gin.Context) {
	clientID := c.Param("id")

	// 检查客户端是否在线
	if !h.remoteOps.IsClientOnline(clientID) {
		ClientNotOnline(c)
		return
	}

	// 解析查询参数
	lines := 100
	if v := c.Query("lines"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			lines = n
		}
	}

	follow := true
	if v := c.Query("follow"); v == "false" {
		follow = false
	}

	level := c.Query("level")

	// 生成会话 ID
	sessionID := uuid.New().String()

	// 启动日志流
	logCh, err := h.remoteOps.StartClientLogStream(clientID, sessionID, lines, follow, level)
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	// 设置 SSE 响应头
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// 获取客户端断开信号
	clientGone := c.Request.Context().Done()

	// 流式传输日志
	c.Stream(func(w io.Writer) bool {
		select {
		case <-clientGone:
			h.remoteOps.StopClientLogStream(sessionID)
			return false
		case entry, ok := <-logCh:
			if !ok {
				return false
			}
			c.SSEvent("log", entry)
			return true
		case <-time.After(30 * time.Second):
			// 发送心跳
			c.SSEvent("heartbeat", gin.H{"ts": time.Now().UnixMilli()})
			return true
		}
	})
}
