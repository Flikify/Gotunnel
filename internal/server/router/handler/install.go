package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/db"
)

type InstallHandler struct {
	app AppInterface
}

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
		ExpiresAt:  now + 3600,
		TunnelPort: h.app.GetServer().GetBindPort(),
	})
}
