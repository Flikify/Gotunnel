package plugin

// 内置协议类型配置模式

// BuiltinRuleSchemas 返回所有内置协议类型的配置模式
func BuiltinRuleSchemas() map[string]RuleSchema {
	return map[string]RuleSchema{
		"tcp": {
			NeedsLocalAddr: true,
			ExtraFields:    nil,
		},
		"udp": {
			NeedsLocalAddr: true,
			ExtraFields:    nil,
		},
		"http": {
			NeedsLocalAddr: false,
			ExtraFields: []ConfigField{
				{
					Key:         "auth_enabled",
					Label:       "启用认证",
					Type:        ConfigFieldBool,
					Default:     "false",
					Description: "是否启用 HTTP Basic 认证",
				},
				{
					Key:         "username",
					Label:       "用户名",
					Type:        ConfigFieldString,
					Description: "HTTP 代理认证用户名",
				},
				{
					Key:         "password",
					Label:       "密码",
					Type:        ConfigFieldPassword,
					Description: "HTTP 代理认证密码",
				},
			},
		},
		"https": {
			NeedsLocalAddr: false,
			ExtraFields: []ConfigField{
				{
					Key:         "auth_enabled",
					Label:       "启用认证",
					Type:        ConfigFieldBool,
					Default:     "false",
					Description: "是否启用 HTTPS 代理认证",
				},
				{
					Key:         "username",
					Label:       "用户名",
					Type:        ConfigFieldString,
					Description: "HTTPS 代理认证用户名",
				},
				{
					Key:         "password",
					Label:       "密码",
					Type:        ConfigFieldPassword,
					Description: "HTTPS 代理认证密码",
				},
			},
		},
		"socks5": {
			NeedsLocalAddr: false,
			ExtraFields: []ConfigField{
				{
					Key:         "auth_enabled",
					Label:       "启用认证",
					Type:        ConfigFieldBool,
					Default:     "false",
					Description: "是否启用 SOCKS5 用户名/密码认证",
				},
				{
					Key:         "username",
					Label:       "用户名",
					Type:        ConfigFieldString,
					Description: "SOCKS5 认证用户名",
				},
				{
					Key:         "password",
					Label:       "密码",
					Type:        ConfigFieldPassword,
					Description: "SOCKS5 认证密码",
				},
			},
		},
	}
}

// GetRuleSchema 获取指定协议类型的配置模式
func GetRuleSchema(proxyType string) *RuleSchema {
	schemas := BuiltinRuleSchemas()
	if schema, ok := schemas[proxyType]; ok {
		return &schema
	}
	return nil
}

// IsBuiltinType 检查是否为内置协议类型
func IsBuiltinType(proxyType string) bool {
	builtinTypes := []string{"tcp", "udp", "http", "https"}
	for _, t := range builtinTypes {
		if t == proxyType {
			return true
		}
	}
	return false
}
