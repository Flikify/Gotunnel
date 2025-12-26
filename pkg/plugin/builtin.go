package plugin

// RegisterBuiltins 注册所有内置 plugins
func RegisterBuiltins(registry *Registry, handlers ...ProxyHandler) error {
	for _, handler := range handlers {
		if err := registry.RegisterBuiltin(handler); err != nil {
			return err
		}
	}
	return nil
}
