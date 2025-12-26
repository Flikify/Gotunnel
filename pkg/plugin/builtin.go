package plugin

// RegisterBuiltins 注册所有内置 plugins
// 注意：此函数需要在调用方导入 builtin 包并手动注册
// 示例:
//   registry := plugin.NewRegistry()
//   registry.RegisterBuiltin(builtin.NewSOCKS5Plugin())
//   registry.RegisterBuiltin(builtin.NewHTTPPlugin())
func RegisterBuiltins(registry *Registry, handlers ...ProxyHandler) error {
	for _, handler := range handlers {
		if err := registry.RegisterBuiltin(handler); err != nil {
			return err
		}
	}
	return nil
}
