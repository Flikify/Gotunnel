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

	// 构建配置值
	config := clientPlugin.Config
	if config == nil {
		config = make(map[string]string)
	}
	// 将 remote_port 加入配置
	if clientPlugin.RemotePort > 0 {
		config["remote_port"] = fmt.Sprintf("%d", clientPlugin.RemotePort)
	}

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
	for i, p := range client.Plugins {
		if p.Name == pluginName {
			// 提取 remote_port 并单独处理
			if portStr, ok := req.Config["remote_port"]; ok {
				var newPort int
				fmt.Sscanf(portStr, "%d", &newPort)
				if newPort > 0 && newPort != client.Plugins[i].RemotePort {
					client.Plugins[i].RemotePort = newPort
					portChanged = true
				}
				delete(req.Config, "remote_port") // 不保存到 Config map
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

	// 保存到数据库
	if err := h.app.GetClientStore().UpdateClient(client); err != nil {
		InternalError(c, err.Error())
		return
	}

	// 如果客户端在线，同步配置
	online, _, _ := h.app.GetServer().GetClientStatus(clientID)
	if online {
		if err := h.app.GetServer().SyncPluginConfigToClient(clientID, pluginName, req.Config); err != nil {
			// 配置已保存，但同步失败，返回警告
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
