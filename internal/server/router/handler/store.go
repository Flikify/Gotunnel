package handler

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/db"
	// removed router import
	"github.com/gotunnel/internal/server/router/dto"
)

// StoreHandler 插件商店处理器
type StoreHandler struct {
	app AppInterface
}

// NewStoreHandler 创建插件商店处理器
func NewStoreHandler(app AppInterface) *StoreHandler {
	return &StoreHandler{app: app}
}

// ListPlugins 获取商店插件列表
// @Summary 获取商店插件
// @Description 从远程插件商店获取可用插件列表
// @Tags 插件商店
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=object{plugins=[]dto.StorePluginInfo}}
// @Failure 502 {object} Response
// @Router /api/store/plugins [get]
func (h *StoreHandler) ListPlugins(c *gin.Context) {
	cfg := h.app.GetConfig()
	storeURL := cfg.PluginStore.GetPluginStoreURL()

	// 从远程 URL 获取插件列表
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(storeURL)
	if err != nil {
		BadGateway(c, "Failed to fetch store: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		BadGateway(c, "Store returned error")
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		InternalError(c, "Failed to read response")
		return
	}

	// 直接返回原始 JSON（已经是数组格式）
	c.Header("Content-Type", "application/json")
	c.Writer.Write([]byte(`{"code":0,"data":{"plugins":`))
	c.Writer.Write(body)
	c.Writer.Write([]byte(`}}`))
}

// Install 从商店安装插件到客户端
// @Summary 安装商店插件
// @Description 从插件商店下载并安装插件到指定客户端
// @Tags 插件商店
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body dto.StoreInstallRequest true "安装请求"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 502 {object} Response
// @Router /api/store/install [post]
func (h *StoreHandler) Install(c *gin.Context) {
	var req dto.StoreInstallRequest
	if !BindJSON(c, &req) {
		return
	}

	// 检查客户端是否在线
	online, _, _ := h.app.GetServer().GetClientStatus(req.ClientID)
	if !online {
		ClientNotOnline(c)
		return
	}

	// 下载插件
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(req.DownloadURL)
	if err != nil {
		BadGateway(c, "Failed to download plugin: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		BadGateway(c, "Plugin download failed with status: "+resp.Status)
		return
	}

	source, err := io.ReadAll(resp.Body)
	if err != nil {
		InternalError(c, "Failed to read plugin: "+err.Error())
		return
	}

	// 下载签名文件
	sigResp, err := client.Get(req.SignatureURL)
	if err != nil {
		BadGateway(c, "Failed to download signature: "+err.Error())
		return
	}
	defer sigResp.Body.Close()

	if sigResp.StatusCode != http.StatusOK {
		BadGateway(c, "Signature download failed with status: "+sigResp.Status)
		return
	}

	signature, err := io.ReadAll(sigResp.Body)
	if err != nil {
		InternalError(c, "Failed to read signature: "+err.Error())
		return
	}

	// 安装到客户端
	installReq := JSPluginInstallRequest{
		PluginName: req.PluginName,
		Source:     string(source),
		Signature:  string(signature),
		RuleName:   req.PluginName,
		RemotePort: req.RemotePort,
		AutoStart:  true,
	}

	if err := h.app.GetServer().InstallJSPluginToClient(req.ClientID, installReq); err != nil {
		InternalError(c, "Failed to install plugin: "+err.Error())
		return
	}

	// 将插件保存到 JSPluginStore（用于客户端重连时恢复）
	jsPlugin := &db.JSPlugin{
		Name:      req.PluginName,
		Source:    string(source),
		Signature: string(signature),
		AutoStart: true,
		Enabled:   true,
	}
	// 尝试保存，忽略错误（可能已存在）
	h.app.GetJSPluginStore().SaveJSPlugin(jsPlugin)

	// 将插件信息保存到客户端记录
	dbClient, err := h.app.GetClientStore().GetClient(req.ClientID)
	if err == nil {
		// 检查插件是否已存在
		exists := false
		for i, p := range dbClient.Plugins {
			if p.Name == req.PluginName {
				dbClient.Plugins[i].Enabled = true
				exists = true
				break
			}
		}
		if !exists {
			dbClient.Plugins = append(dbClient.Plugins, db.ClientPlugin{
				Name:    req.PluginName,
				Version: "1.0.0",
				Enabled: true,
			})
		}
		h.app.GetClientStore().UpdateClient(dbClient)
	}

	Success(c, gin.H{
		"status": "ok",
		"plugin": req.PluginName,
		"client": req.ClientID,
	})
}
