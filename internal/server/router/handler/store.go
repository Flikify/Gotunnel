package handler

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/router/dto"
	"github.com/gotunnel/pkg/protocol"
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

	// 检查插件是否已存在，决定使用已有 ID 还是生成新 ID
	pluginID := ""
	dbClient, err := h.app.GetClientStore().GetClient(req.ClientID)
	if err == nil {
		for _, p := range dbClient.Plugins {
			if p.Name == req.PluginName && p.ID != "" {
				pluginID = p.ID
				break
			}
		}
	}
	if pluginID == "" {
		pluginID = uuid.New().String()
	}

	// 安装到客户端
	installReq := JSPluginInstallRequest{
		PluginID:   pluginID,
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
	// 重新获取 dbClient（可能已被修改）
	dbClient, err = h.app.GetClientStore().GetClient(req.ClientID)
	if err == nil {
		// 检查插件是否已存在（通过名称匹配）
		pluginExists := false
		for i, p := range dbClient.Plugins {
			if p.Name == req.PluginName {
				dbClient.Plugins[i].Enabled = true
				dbClient.Plugins[i].RemotePort = req.RemotePort
				// 确保有 ID
				if dbClient.Plugins[i].ID == "" {
					dbClient.Plugins[i].ID = pluginID
				}
				pluginExists = true
				break
			}
		}
		if !pluginExists {
			version := req.Version
			if version == "" {
				version = "1.0.0"
			}
			// 转换 ConfigSchema
			var configSchema []db.ConfigField
			for _, f := range req.ConfigSchema {
				configSchema = append(configSchema, db.ConfigField{
					Key:         f.Key,
					Label:       f.Label,
					Type:        f.Type,
					Default:     f.Default,
					Required:    f.Required,
					Options:     f.Options,
					Description: f.Description,
				})
			}
			dbClient.Plugins = append(dbClient.Plugins, db.ClientPlugin{
				ID:           pluginID,
				Name:         req.PluginName,
				Version:      version,
				Enabled:      true,
				RemotePort:   req.RemotePort,
				ConfigSchema: configSchema,
			})
		}

		// 自动创建代理规则（如果指定了端口）
		if req.RemotePort > 0 {
			ruleExists := false
			for i, r := range dbClient.Rules {
				if r.Name == req.PluginName {
					// 更新现有规则
					dbClient.Rules[i].Type = req.PluginName
					dbClient.Rules[i].RemotePort = req.RemotePort
					dbClient.Rules[i].Enabled = boolPtr(true)
					dbClient.Rules[i].AuthEnabled = req.AuthEnabled
					dbClient.Rules[i].AuthUsername = req.AuthUsername
					dbClient.Rules[i].AuthPassword = req.AuthPassword
					ruleExists = true
					break
				}
			}
			if !ruleExists {
				// 创建新规则
				dbClient.Rules = append(dbClient.Rules, protocol.ProxyRule{
					Name:         req.PluginName,
					Type:         req.PluginName,
					RemotePort:   req.RemotePort,
					Enabled:      boolPtr(true),
					AuthEnabled:  req.AuthEnabled,
					AuthUsername: req.AuthUsername,
					AuthPassword: req.AuthPassword,
				})
			}
		}

		h.app.GetClientStore().UpdateClient(dbClient)
	}

	// 启动服务端监听器（让外部用户可以通过 RemotePort 访问插件）
	if req.RemotePort > 0 {
		pluginRule := protocol.ProxyRule{
			Name:         req.PluginName,
			Type:         req.PluginName, // 使用插件名作为类型，让 isClientPlugin 识别
			RemotePort:   req.RemotePort,
			Enabled:      boolPtr(true),
			AuthEnabled:  req.AuthEnabled,
			AuthUsername: req.AuthUsername,
			AuthPassword: req.AuthPassword,
		}
		// 启动监听器（忽略错误，可能端口已被占用）
		h.app.GetServer().StartPluginRule(req.ClientID, pluginRule)
	}

	Success(c, gin.H{
		"status":    "ok",
		"plugin":    req.PluginName,
		"plugin_id": pluginID,
		"client":    req.ClientID,
	})
}

// boolPtr 返回 bool 值的指针
func boolPtr(b bool) *bool {
	return &b
}
