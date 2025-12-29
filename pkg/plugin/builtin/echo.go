package builtin

import (
	"io"
	"log"
	"net"
	"sync"

	"github.com/gotunnel/pkg/plugin"
)

func init() {
	RegisterClient(NewEchoPlugin())
}

// EchoPlugin 回显插件 - 客户端插件示例
type EchoPlugin struct {
	config   map[string]string
	listener net.Listener
	running  bool
	mu       sync.Mutex
}

// NewEchoPlugin 创建 Echo 插件
func NewEchoPlugin() *EchoPlugin {
	return &EchoPlugin{}
}

// Metadata 返回插件信息
func (p *EchoPlugin) Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name:        "echo",
		Version:     "1.0.0",
		Type:        plugin.PluginTypeApp,
		Source:      plugin.PluginSourceBuiltin,
		RunAt:       plugin.SideClient,
		Description: "Echo server (client plugin example)",
		Author:      "GoTunnel",
		RuleSchema: &plugin.RuleSchema{
			NeedsLocalAddr: false,
		},
	}
}

// Init 初始化插件
func (p *EchoPlugin) Init(config map[string]string) error {
	p.config = config
	return nil
}

// Start 启动服务
func (p *EchoPlugin) Start() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return "", nil
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	p.listener = ln
	p.running = true

	log.Printf("[Echo] Started on %s", ln.Addr().String())
	return ln.Addr().String(), nil
}

// HandleConn 处理连接
func (p *EchoPlugin) HandleConn(conn net.Conn) error {
	defer conn.Close()
	log.Printf("[Echo] New connection from tunnel")
	_, err := io.Copy(conn, conn)
	return err
}

// Stop 停止服务
func (p *EchoPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	if p.listener != nil {
		p.listener.Close()
	}
	p.running = false
	log.Printf("[Echo] Stopped")
	return nil
}
