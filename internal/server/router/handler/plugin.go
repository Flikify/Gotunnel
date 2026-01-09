package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/router/dto"
	"github.com/gotunnel/pkg/plugin"
)

// PluginHandler 插件处理器
type PluginHandler struct {
	app AppInterface
}

// NewPluginHandler 创建插件处理器
func NewPluginHandler(app AppInterface) *PluginHandler {
	return &PluginHandler{app: app}
}

// List 获取插件列表
// @Summary 获取所有插件
// @Description 返回服务端所有注册的插件
// @Tags 插件
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=[]dto.PluginInfo}
// @Router /api/plugins [get]
func (h *PluginHandler) List(c *gin.Context) {
	plugins := h.app.GetServer().GetPluginList()

	result := make([]dto.PluginInfo, len(plugins))
	for i, p := range plugins {
		result[i] = dto.PluginInfo{
			Name:        p.Name,
			Version:     p.Version,
			Type:        p.Type,
			Description: p.Description,
			Source:      p.Source,
			Icon:        p.Icon,
			Enabled:     p.Enabled,
		}
		if p.RuleSchema != nil {
			result[i].RuleSchema = &dto.RuleSchema{
				NeedsLocalAddr: p.RuleSchema.NeedsLocalAddr,
				ExtraFields:    convertRouterConfigFields(p.RuleSchema.ExtraFields),
			}
		}
	}

	Success(c, result)
}

// Enable 启用插件
// @Summary 启用插件
// @Description 启用指定插件
// @Tags 插件
// @Produce json
// @Security Bearer
// @Param name path string true "插件名称"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Router /api/plugin/{name}/enable [post]
func (h *PluginHandler) Enable(c *gin.Context) {
	name := c.Param("name")

	if err := h.app.GetServer().EnablePlugin(name); err != nil {
		BadRequest(c, err.Error())
		return
	}

	Success(c, gin.H{"status": "ok"})
}

// Disable 禁用插件
// @Summary 禁用插件
// @Description 禁用指定插件
// @Tags 插件
// @Produce json
// @Security Bearer
// @Param name path string true "插件名称"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Router /api/plugin/{name}/disable [post]
func (h *PluginHandler) Disable(c *gin.Context) {
	name := c.Param("name")

	if err := h.app.GetServer().DisablePlugin(name); err != nil {
		BadRequest(c, err.Error())
		return
	}

	Success(c, gin.H{"status": "ok"})
}

// GetRuleSchemas 获取规则配置模式
// @Summary 获取规则模式
// @Description 返回所有协议类型的配置模式
// @Tags 插件
// @Produce json
// @Security Bearer
// @Success 200 {object} Response{data=map[string]dto.RuleSchema}
// @Router /api/rule-schemas [get]
func (h *PluginHandler) GetRuleSchemas(c *gin.Context) {
	// 获取内置协议模式
	schemas := make(map[string]dto.RuleSchema)
	for name, schema := range plugin.BuiltinRuleSchemas() {
		schemas[name] = dto.RuleSchema{
			NeedsLocalAddr: schema.NeedsLocalAddr,
			ExtraFields:    convertConfigFields(schema.ExtraFields),
		}
	}

	// 添加已注册插件的模式
	plugins := h.app.GetServer().GetPluginList()
	for _, p := range plugins {
		if p.RuleSchema != nil {
			schemas[p.Name] = dto.RuleSchema{
				NeedsLocalAddr: p.RuleSchema.NeedsLocalAddr,
				ExtraFields:    convertRouterConfigFields(p.RuleSchema.ExtraFields),
			}
		}
	}

	Success(c, schemas)
}

// GetClientConfig 获取客户端插件配置
// @Summary 获取客户端插件配置
// @Description 获取客户端上指定插件的配置
// @Tags 插件
// @Produce json
// @Security Bearer
// @Param clientID path string true "客户端ID"
// @Param pluginName path string true "插件名称"
// @Success 200 {object} Response{data=dto.PluginConfigResponse}
// @Failure 404 {object} Response
// @Router /api/client-plugin/{clientID}/{pluginName}/config [get]
func (h *PluginHandler) GetClientConfig(c *gin.Context) {
	clientID := c.Param("clientID")
	pluginName := c.Param("pluginName")

	client, err := h.app.GetClientStore().GetClient(clientID)
	if err != nil {
		NotFound(c, "client not found")
		return
	}

	// 查找客户端的插件
	var clientPlugin *db.ClientPlugin
	for i, p := range client.Plugins {
		if p.Name == pluginName {
			clientPlugin = &client.Plugins[i]
			break
		}
	}

	if clientPlugin == nil {
		NotFound(c, "plugin not installed on client")
		return
	}

	var schemaFields []dto.ConfigField

	// 优先使用客户端插件保存的 ConfigSchema
	if len(clientPlugin.ConfigSchema) > 0 {
		for _, f := range clientPlugin.ConfigSchema {
			schemaFields = append(schemaFields, dto.ConfigField{
				Key:         f.Key,
				Label:       f.Label,
				Type:        f.Type,
				Default:     f.Default,
				Required:    f.Required,
				Options:     f.Options,
				Description: f.Description,
			})
		}
	} else {
		// 尝试从内置插件获取配置模式
		schema, err := h.app.GetServer().GetPluginConfigSchema(pluginName)
		if err != nil {
			// 如果内置插件中找不到，尝试从 JS 插件获取
			jsPlugin, jsErr := h.app.GetJSPluginStore().GetJSPlugin(pluginName)
			if jsErr == nil {
				// 使用 JS 插件的 config 作为动态 schema
				for key := range jsPlugin.Config {
					schemaFields = append(schemaFields, dto.ConfigField{
						Key:   key,
						Label: key,
						Type:  "string",
					})
				}
			}
		} else {
			schemaFields = convertRouterConfigFields(schema)
		}
	}

	// 添加 remote_port 作为系统配置字段（始终显示）
	schemaFields = append([]dto.ConfigField{{
		Key:         "remote_port",
		Label:       "远程端口",
		Type:        "number",
		Description: "服务端监听端口，修改后需重启插件生效",
	}}, schemaFields...)

	// 添加 Auth 配置字段
	schemaFields = append(schemaFields, dto.ConfigField{
		Key:         "auth_enabled",
		Label:       "启用认证",
		Type:        "boolean",
		Description: "启用 HTTP Basic Auth 保护",
	}, dto.ConfigField{
		Key:         "auth_username",
		Label:       "认证用户名",
		Type:        "string",
		Description: "HTTP Basic Auth 用户名",
	}, dto.ConfigField{
		Key:         "auth_password",
		Label:       "认证密码",
		Type:        "password",
		Description: "HTTP Basic Auth 密码",
	})

	// 构建配置值
	config := clientPlugin.Config
	if config == nil {
		config = make(map[string]string)
	}
	// 将 remote_port 加入配置
	if clientPlugin.RemotePort > 0 {
		config["remote_port"] = fmt.Sprintf("%d", clientPlugin.RemotePort)
	}
	// 将 Auth 配置加入
	if clientPlugin.AuthEnabled {
		config["auth_enabled"] = "true"
	} else {
		config["auth_enabled"] = "false"
	}
	config["auth_username"] = clientPlugin.AuthUsername
	config["auth_password"] = clientPlugin.AuthPassword

	Success(c, dto.PluginConfigResponse{
		PluginName: pluginName,
		Schema:     schemaFields,
		Config:     config,
	})
}

// UpdateClientConfig 更新客户端插件配置
// @Summary 更新客户端插件配置
// @Description 更新客户端上指定插件的配置
// @Tags 插件
// @Accept json
// @Produce json
// @Security Bearer
// @Param clientID path string true "客户端ID"
// @Param pluginName path string true "插件名称"
// @Param request body dto.PluginConfigRequest true "配置内容"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /api/client-plugin/{clientID}/{pluginName}/config [put]
func (h *PluginHandler) UpdateClientConfig(c *gin.Context) {
	clientID := c.Param("clientID")
	pluginName := c.Param("pluginName")

	var req dto.PluginConfigRequest
	if !BindJSON(c, &req) {
		return
	}

	client, err := h.app.GetClientStore().GetClient(clientID)
	if err != nil {
		NotFound(c, "client not found")
		return
	}

	// 更新插件配置
	found := false
	portChanged := false
	authChanged := false
	var oldPort, newPort int
	for i, p := range client.Plugins {
		if p.Name == pluginName {
			oldPort = client.Plugins[i].RemotePort
			// 提取 remote_port 并单独处理
			if portStr, ok := req.Config["remote_port"]; ok {
				fmt.Sscanf(portStr, "%d", &newPort)
				if newPort > 0 && newPort != oldPort {
					// 检查新端口是否可用
					if !h.app.GetServer().IsPortAvailable(newPort, clientID) {
						BadRequest(c, fmt.Sprintf("port %d is already in use", newPort))
						return
					}
					client.Plugins[i].RemotePort = newPort
					portChanged = true
				}
				delete(req.Config, "remote_port") // 不保存到 Config map
			}
			// 提取 Auth 配置并单独处理
			if authEnabledStr, ok := req.Config["auth_enabled"]; ok {
				newAuthEnabled := authEnabledStr == "true"
				if newAuthEnabled != client.Plugins[i].AuthEnabled {
					client.Plugins[i].AuthEnabled = newAuthEnabled
					authChanged = true
				}
				delete(req.Config, "auth_enabled")
			}
			if authUsername, ok := req.Config["auth_username"]; ok {
				if authUsername != client.Plugins[i].AuthUsername {
					client.Plugins[i].AuthUsername = authUsername
					authChanged = true
				}
				delete(req.Config, "auth_username")
			}
			if authPassword, ok := req.Config["auth_password"]; ok {
				if authPassword != client.Plugins[i].AuthPassword {
					client.Plugins[i].AuthPassword = authPassword
					authChanged = true
				}
				delete(req.Config, "auth_password")
			}
			client.Plugins[i].Config = req.Config
			found = true
			break
		}
	}

	if !found {
		NotFound(c, "plugin not installed on client")
		return
	}

	// 如果端口变更，同步更新代理规则
	if portChanged {
		for i, r := range client.Rules {
			if r.Name == pluginName && r.PluginManaged {
				client.Rules[i].RemotePort = newPort
				break
			}
		}
		// 停止旧端口监听器
		if oldPort > 0 {
			h.app.GetServer().StopPluginRule(clientID, oldPort)
		}
	}

	// 如果 Auth 配置变更，同步更新代理规则
	if authChanged {
		for i, p := range client.Plugins {
			if p.Name == pluginName {
				for j, r := range client.Rules {
					if r.Name == pluginName && r.PluginManaged {
						client.Rules[j].AuthEnabled = client.Plugins[i].AuthEnabled
						client.Rules[j].AuthUsername = client.Plugins[i].AuthUsername
						client.Rules[j].AuthPassword = client.Plugins[i].AuthPassword
						break
					}
				}
				break
			}
		}
	}

	// 保存到数据库
	if err := h.app.GetClientStore().UpdateClient(client); err != nil {
		InternalError(c, err.Error())
		return
	}

	// 如果客户端在线，同步配置
	online, _, _ := h.app.GetServer().GetClientStatus(clientID)
	if online {
		if err := h.app.GetServer().SyncPluginConfigToClient(clientID, pluginName, req.Config); err != nil {
			PartialSuccess(c, gin.H{"status": "partial", "port_changed": portChanged}, "config saved but sync failed: "+err.Error())
			return
		}
	}

	Success(c, gin.H{"status": "ok", "port_changed": portChanged})
}

// convertConfigFields 转换插件配置字段到 DTO
func convertConfigFields(fields []plugin.ConfigField) []dto.ConfigField {
	result := make([]dto.ConfigField, len(fields))
	for i, f := range fields {
		result[i] = dto.ConfigField{
			Key:         f.Key,
			Label:       f.Label,
			Type:        string(f.Type),
			Default:     f.Default,
			Required:    f.Required,
			Options:     f.Options,
			Description: f.Description,
		}
	}
	return result
}

// convertRouterConfigFields 转换 ConfigField 到 dto.ConfigField
func convertRouterConfigFields(fields []ConfigField) []dto.ConfigField {
	result := make([]dto.ConfigField, len(fields))
	for i, f := range fields {
		result[i] = dto.ConfigField{
			Key:         f.Key,
			Label:       f.Label,
			Type:        f.Type,
			Default:     f.Default,
			Required:    f.Required,
			Options:     f.Options,
			Description: f.Description,
		}
	}
	return result
}
