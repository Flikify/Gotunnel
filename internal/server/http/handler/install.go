package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/gotunnel/internal/server/storage/sqlite"
)

type InstallHandler struct {
	tokenStore db.InstallTokenStore
	serverInfo ServerInfoInterface
}

const (
	installTokenHeader = "X-GoTunnel-Install-Token"
	installTokenTTL    = 3600
)

func NewInstallHandler(tokenStore db.InstallTokenStore, serverInfo ServerInfoInterface) *InstallHandler {
	return &InstallHandler{
		tokenStore: tokenStore,
		serverInfo: serverInfo,
	}
}

type InstallCommandResponse struct {
	Token      string `json:"token"`
	ExpiresAt  int64  `json:"expires_at"`
	TunnelPort int    `json:"tunnel_port"`
}

// GenerateInstallCommand creates a one-time install token and returns
// the tunnel port so the frontend can build a host-aware command.
//
// @Summary Generate install command payload
// @Tags install
// @Produce json
// @Success 200 {object} Response{data=InstallCommandResponse}
// @Router /api/installations/actions/command [post]
func (h *InstallHandler) GenerateInstallCommand(c *gin.Context) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		InternalError(c, "failed to generate token")
		return
	}

	token := hex.EncodeToString(tokenBytes)
	now := time.Now().Unix()

	installToken := &db.InstallToken{
		Token:     token,
		CreatedAt: now,
		Used:      false,
	}

	if err := h.tokenStore.CreateInstallToken(installToken); err != nil {
		InternalError(c, "failed to persist token")
		return
	}

	Success(c, InstallCommandResponse{
		Token:      token,
		ExpiresAt:  now + installTokenTTL,
		TunnelPort: h.serverInfo.GetBindPort(),
	})
}

func (h *InstallHandler) ServeShellScript(c *gin.Context) {
	if !h.validateInstallToken(c) {
		return
	}

	applyInstallSecurityHeaders(c)
	c.Header("Content-Type", "text/x-shellscript; charset=utf-8")
	c.String(http.StatusOK, shellInstallScript)
}

func (h *InstallHandler) ServePowerShellScript(c *gin.Context) {
	if !h.validateInstallToken(c) {
		return
	}

	applyInstallSecurityHeaders(c)
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, powerShellInstallScript)
}

func (h *InstallHandler) validateInstallToken(c *gin.Context) bool {
	token := strings.TrimSpace(c.GetHeader(installTokenHeader))
	if token == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return false
	}

	installToken, err := h.tokenStore.GetInstallToken(token)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return false
	}

	if installToken.Used || time.Now().Unix()-installToken.CreatedAt >= installTokenTTL {
		c.AbortWithStatus(http.StatusNotFound)
		return false
	}

	return true
}

func applyInstallSecurityHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Robots-Tag", "noindex, nofollow, noarchive")
}
