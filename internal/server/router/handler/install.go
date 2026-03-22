package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/db"
)

type InstallHandler struct {
	app AppInterface
}

const (
	installTokenHeader = "X-GoTunnel-Install-Token"
	installTokenTTL    = 3600
)

func NewInstallHandler(app AppInterface) *InstallHandler {
	return &InstallHandler{app: app}
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
// @Success 200 {object} InstallCommandResponse
// @Router /api/install/generate [post]
func (h *InstallHandler) GenerateInstallCommand(c *gin.Context) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	token := hex.EncodeToString(tokenBytes)
	now := time.Now().Unix()

	installToken := &db.InstallToken{
		Token:     token,
		CreatedAt: now,
		Used:      false,
	}

	store, ok := h.app.GetClientStore().(db.InstallTokenStore)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "install token store is not supported"})
		return
	}

	if err := store.CreateInstallToken(installToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist token"})
		return
	}

	c.JSON(http.StatusOK, InstallCommandResponse{
		Token:      token,
		ExpiresAt:  now + installTokenTTL,
		TunnelPort: h.app.GetServer().GetBindPort(),
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

func (h *InstallHandler) DownloadClient(c *gin.Context) {
	if !h.validateInstallToken(c) {
		return
	}

	osName := c.Query("os")
	arch := c.Query("arch")

	updateInfo, err := checkClientUpdateForPlatform(osName, arch)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve client package"})
		return
	}
	if updateInfo.DownloadURL == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "no client package found for this platform"})
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, updateInfo.DownloadURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create download request"})
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to download client package"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("upstream returned %s", resp.Status)})
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	applyInstallSecurityHeaders(c)
	c.Header("Content-Type", contentType)
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		c.Header("Content-Length", contentLength)
	}
	if updateInfo.AssetName != "" {
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, updateInfo.AssetName))
	}

	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, resp.Body)
}

func (h *InstallHandler) validateInstallToken(c *gin.Context) bool {
	token := strings.TrimSpace(c.GetHeader(installTokenHeader))
	if token == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return false
	}

	store, ok := h.app.GetClientStore().(db.InstallTokenStore)
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return false
	}

	installToken, err := store.GetInstallToken(token)
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
