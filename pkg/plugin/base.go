package plugin

import (
	"fmt"
	"log"
	"sync"
)

// =============================================================================
// 基础实现 - 提取公共代码
// =============================================================================

// baseAPI 包含服务端和客户端共享的基础功能
type baseAPI struct {
	pluginName string
	config     map[string]string
	configMu   sync.RWMutex

	eventHandlers map[EventType][]EventHandler
	eventMu       sync.RWMutex
}

// newBaseAPI 创建基础 API
func newBaseAPI(pluginName string, config map[string]string) *baseAPI {
	cfg := config
	if cfg == nil {
		cfg = make(map[string]string)
	}
	return &baseAPI{
		pluginName:    pluginName,
		config:        cfg,
		eventHandlers: make(map[EventType][]EventHandler),
	}
}

// Log 记录日志
func (b *baseAPI) Log(level LogLevel, format string, args ...interface{}) {
	prefix := fmt.Sprintf("[Plugin:%s] ", b.pluginName)
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s%s", prefix, msg)
}

// GetConfig 获取配置值
func (b *baseAPI) GetConfig(key string) string {
	b.configMu.RLock()
	defer b.configMu.RUnlock()
	return b.config[key]
}

// SetConfig 设置配置值
func (b *baseAPI) SetConfig(key, value string) {
	b.configMu.Lock()
	defer b.configMu.Unlock()
	b.config[key] = value
}

// OnEvent 订阅事件
func (b *baseAPI) OnEvent(eventType EventType, handler EventHandler) {
	b.eventMu.Lock()
	defer b.eventMu.Unlock()
	b.eventHandlers[eventType] = append(b.eventHandlers[eventType], handler)
}

// EmitEvent 发送事件（复制切片避免竞态条件）
func (b *baseAPI) EmitEvent(event *Event) {
	b.eventMu.RLock()
	handlers := make([]EventHandler, len(b.eventHandlers[event.Type]))
	copy(handlers, b.eventHandlers[event.Type])
	b.eventMu.RUnlock()

	for _, handler := range handlers {
		go handler(event)
	}
}

// getPluginName 获取插件名称
func (b *baseAPI) getPluginName() string {
	return b.pluginName
}

// getConfigMap 获取配置副本
func (b *baseAPI) getConfigMap() map[string]string {
	b.configMu.RLock()
	defer b.configMu.RUnlock()
	result := make(map[string]string, len(b.config))
	for k, v := range b.config {
		result[k] = v
	}
	return result
}
