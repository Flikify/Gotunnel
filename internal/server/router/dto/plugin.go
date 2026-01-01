package dto

// PluginConfigRequest 更新插件配置请求
// @Description 更新客户端插件配置
type PluginConfigRequest struct {
	Config map[string]string `json:"config" binding:"required"`
}

// PluginConfigResponse 插件配置响应
// @Description 插件配置详情
type PluginConfigResponse struct {
	PluginName string        `json:"plugin_name"`
	Schema     []ConfigField `json:"schema"`
	Config     map[string]string `json:"config"`
}

// ConfigField 配置字段定义
// @Description 配置表单字段
type ConfigField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Default     string   `json:"default,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Options     []string `json:"options,omitempty"`
	Description string   `json:"description,omitempty"`
}

// RuleSchema 规则表单模式
// @Description 代理规则的配置模式
type RuleSchema struct {
	NeedsLocalAddr bool          `json:"needs_local_addr"`
	ExtraFields    []ConfigField `json:"extra_fields,omitempty"`
}

// PluginInfo 插件信息
// @Description 服务端插件信息
type PluginInfo struct {
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Source      string      `json:"source"`
	Icon        string      `json:"icon,omitempty"`
	Enabled     bool        `json:"enabled"`
	RuleSchema  *RuleSchema `json:"rule_schema,omitempty"`
}

// JSPluginCreateRequest 创建 JS 插件请求
// @Description 创建新的 JS 插件
type JSPluginCreateRequest struct {
	Name        string            `json:"name" binding:"required,min=1,max=64"`
	Source      string            `json:"source" binding:"required"`
	Signature   string            `json:"signature"`
	Description string            `json:"description" binding:"max=500"`
	Author      string            `json:"author" binding:"max=64"`
	Config      map[string]string `json:"config"`
	AutoStart   bool              `json:"auto_start"`
}

// JSPluginUpdateRequest 更新 JS 插件请求
// @Description 更新 JS 插件
type JSPluginUpdateRequest struct {
	Source      string            `json:"source"`
	Signature   string            `json:"signature"`
	Description string            `json:"description" binding:"max=500"`
	Author      string            `json:"author" binding:"max=64"`
	Config      map[string]string `json:"config"`
	AutoStart   bool              `json:"auto_start"`
	Enabled     bool              `json:"enabled"`
}

// JSPluginInstallRequest JS 插件安装请求
// @Description 安装 JS 插件到客户端
type JSPluginInstallRequest struct {
	PluginName string            `json:"plugin_name" binding:"required"`
	Source     string            `json:"source" binding:"required"`
	Signature  string            `json:"signature"`
	RuleName   string            `json:"rule_name"`
	RemotePort int               `json:"remote_port"`
	Config     map[string]string `json:"config"`
	AutoStart  bool              `json:"auto_start"`
}

// StorePluginInfo 扩展商店插件信息
// @Description 插件商店中的插件信息
type StorePluginInfo struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Type         string `json:"type"`
	Description  string `json:"description"`
	Author       string `json:"author"`
	Icon         string `json:"icon,omitempty"`
	DownloadURL  string `json:"download_url,omitempty"`
	SignatureURL string `json:"signature_url,omitempty"`
}

// StoreInstallRequest 从商店安装插件请求
// @Description 从插件商店安装插件到客户端
type StoreInstallRequest struct {
	PluginName   string `json:"plugin_name" binding:"required"`
	DownloadURL  string `json:"download_url" binding:"required,url"`
	SignatureURL string `json:"signature_url" binding:"required,url"`
	ClientID     string `json:"client_id" binding:"required"`
}
