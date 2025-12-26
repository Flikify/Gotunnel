package wasm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gotunnel/pkg/plugin"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// WASMPlugin 封装 WASM 模块作为 ProxyHandler
type WASMPlugin struct {
	name     string
	metadata plugin.PluginMetadata
	runtime  *Runtime
	compiled wazero.CompiledModule
	config   map[string]string
}

// NewWASMPlugin 从 WASM 字节创建 plugin
func NewWASMPlugin(ctx context.Context, rt *Runtime, name string, wasmBytes []byte) (*WASMPlugin, error) {
	compiled, err := rt.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile module: %w", err)
	}

	p := &WASMPlugin{
		name:     name,
		runtime:  rt,
		compiled: compiled,
	}

	// 尝试获取元数据
	if err := p.loadMetadata(ctx); err != nil {
		// 使用默认元数据
		p.metadata = plugin.PluginMetadata{
			Name:   name,
			Type:   plugin.PluginTypeProxy,
			Source: plugin.PluginSourceWASM,
		}
	}

	return p, nil
}

// loadMetadata 从 WASM 模块加载元数据
func (p *WASMPlugin) loadMetadata(ctx context.Context) error {
	// 创建临时实例获取元数据
	inst, err := p.runtime.runtime.InstantiateModule(ctx, p.compiled, wazero.NewModuleConfig())
	if err != nil {
		return err
	}
	defer inst.Close(ctx)

	metadataFn := inst.ExportedFunction("metadata")
	if metadataFn == nil {
		return fmt.Errorf("metadata function not exported")
	}

	allocFn := inst.ExportedFunction("alloc")
	if allocFn == nil {
		return fmt.Errorf("alloc function not exported")
	}

	// 分配缓冲区
	results, err := allocFn.Call(ctx, 1024)
	if err != nil {
		return err
	}
	bufPtr := uint32(results[0])

	// 调用 metadata 函数
	results, err = metadataFn.Call(ctx, uint64(bufPtr), 1024)
	if err != nil {
		return err
	}
	actualLen := uint32(results[0])

	// 读取元数据
	mem := inst.Memory()
	data, ok := mem.Read(bufPtr, actualLen)
	if !ok {
		return fmt.Errorf("failed to read metadata")
	}

	return json.Unmarshal(data, &p.metadata)
}

// Metadata 返回 plugin 信息
func (p *WASMPlugin) Metadata() plugin.PluginMetadata {
	return p.metadata
}

// Init 初始化 plugin
func (p *WASMPlugin) Init(config map[string]string) error {
	p.config = config
	return nil
}

// HandleConn 处理连接
func (p *WASMPlugin) HandleConn(conn interface{}, dialer plugin.Dialer) error {
	// WASM plugin 的连接处理需要更复杂的实现
	// 这里提供基础框架，实际实现需要注册 host functions
	return fmt.Errorf("WASM plugin HandleConn not fully implemented")
}

// Close 关闭 plugin
func (p *WASMPlugin) Close() error {
	return p.compiled.Close(context.Background())
}

// RegisterHostFunctions 注册 host functions 到 wazero 运行时
func RegisterHostFunctions(ctx context.Context, r wazero.Runtime) (wazero.CompiledModule, error) {
	return r.NewHostModuleBuilder("env").
		NewFunctionBuilder().
		WithFunc(hostLog).
		Export("log").
		NewFunctionBuilder().
		WithFunc(hostNow).
		Export("now").
		Compile(ctx)
}

// host function 实现
func hostLog(ctx context.Context, m api.Module, level uint32, msgPtr, msgLen uint32) {
	data, ok := m.Memory().Read(msgPtr, msgLen)
	if !ok {
		return
	}
	prefix := "[WASM]"
	switch plugin.LogLevel(level) {
	case plugin.LogDebug:
		prefix = "[WASM DEBUG]"
	case plugin.LogInfo:
		prefix = "[WASM INFO]"
	case plugin.LogWarn:
		prefix = "[WASM WARN]"
	case plugin.LogError:
		prefix = "[WASM ERROR]"
	}
	fmt.Printf("%s %s\n", prefix, string(data))
}

func hostNow(ctx context.Context) int64 {
	return ctx.Value("now").(func() int64)()
}
