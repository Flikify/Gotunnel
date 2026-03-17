package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/db"
)

// InstallHandler 安装处理器
type InstallHandler struct {
	app AppInterface
}

// NewInstallHandler 创建安装处理器
func NewInstallHandler(app AppInterface) *InstallHandler {
	return &InstallHandler{app: app}
}

// GenerateInstallCommandRequest 生成安装命令请求
type GenerateInstallCommandRequest struct {
	ClientID string `json:"client_id" binding:"required"`
}

// InstallCommandResponse 安装命令响应
type InstallCommandResponse struct {
	Token       string            `json:"token"`
	Commands    map[string]string `json:"commands"`
	ExpiresAt   int64             `json:"expires_at"`
	ServerAddr  string            `json:"server_addr"`
}

// GenerateInstallCommand 生成安装命令
// @Summary 生成客户端安装命令
// @Tags install
// @Accept json
// @Produce json
// @Param body body GenerateInstallCommandRequest true "客户端ID"
// @Success 200 {object} InstallCommandResponse
// @Router /api/install/generate [post]
func (h *InstallHandler) GenerateInstallCommand(c *gin.Context) {
	var req GenerateInstallCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 生成随机token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	// 保存到数据库
	now := time.Now().Unix()
	installToken := &db.InstallToken{
		Token:     token,
		ClientID:  req.ClientID,
		CreatedAt: now,
		Used:      false,
	}

	store, ok := h.app.GetClientStore().(db.InstallTokenStore)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "存储不支持安装token"})
		return
	}

	if err := store.CreateInstallToken(installToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存token失败"})
		return
	}

	// 获取服务器地址
	serverAddr := fmt.Sprintf("%s:%d", h.app.GetConfig().Server.BindAddr, h.app.GetServer().GetBindPort())
	if h.app.GetConfig().Server.BindAddr == "" || h.app.GetConfig().Server.BindAddr == "0.0.0.0" {
		serverAddr = fmt.Sprintf("your-server-ip:%d", h.app.GetServer().GetBindPort())
	}

	// 生成安装命令
	expiresAt := now + 3600 // 1小时过期
	tlsFlag := ""
	if h.app.GetConfig().Server.TLSDisabled {
		tlsFlag = " -no-tls"
	}

	commands := map[string]string{
		"linux": fmt.Sprintf("curl -fsSL https://raw.githubusercontent.com/gotunnel/gotunnel/main/scripts/install.sh | bash -s -- -s %s -t %s -id %s%s",
			serverAddr, token, req.ClientID, tlsFlag),
		"macos": fmt.Sprintf("curl -fsSL https://raw.githubusercontent.com/gotunnel/gotunnel/main/scripts/install.sh | bash -s -- -s %s -t %s -id %s%s",
			serverAddr, token, req.ClientID, tlsFlag),
		"windows": fmt.Sprintf("powershell -c \"irm https://raw.githubusercontent.com/gotunnel/gotunnel/main/scripts/install.ps1 | iex; Install-GoTunnel -Server '%s' -Token '%s' -ClientID '%s'%s\"",
			serverAddr, token, req.ClientID, tlsFlag),
	}

	c.JSON(http.StatusOK, InstallCommandResponse{
		Token:      token,
		Commands:   commands,
		ExpiresAt:  expiresAt,
		ServerAddr: serverAddr,
	})
}
