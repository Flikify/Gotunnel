package builtin

import (
	"io"
	"log"
	"net"

	"github.com/gotunnel/pkg/plugin"
)

func init() {
	Register(NewVNCPlugin())
}

// VNCPlugin VNC 远程桌面插件
type VNCPlugin struct {
	config map[string]string
}

// NewVNCPlugin 创建 VNC plugin
func NewVNCPlugin() *VNCPlugin {
	return &VNCPlugin{}
}

// Metadata 返回 plugin 信息
func (p *VNCPlugin) Metadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name:        "vnc",
		Version:     "1.0.0",
		Type:        plugin.PluginTypeApp,
		Source:      plugin.PluginSourceBuiltin,
		Description: "VNC remote desktop relay (connects to client's local VNC server)",
		Author:      "GoTunnel",
		Capabilities: []string{
			"dial", "read", "write", "close",
		},
	}
}

// Init 初始化 plugin
func (p *VNCPlugin) Init(config map[string]string) error {
	p.config = config
	return nil
}

// HandleConn 处理 VNC 连接
// 将外部 VNC 客户端连接转发到客户端本地的 VNC 服务
func (p *VNCPlugin) HandleConn(conn net.Conn, dialer plugin.Dialer) error {
	defer conn.Close()

	// 默认连接客户端本地的 VNC 服务 (5900)
	vncAddr := "127.0.0.1:5900"
	if addr, ok := p.config["vnc_addr"]; ok && addr != "" {
		vncAddr = addr
	}

	log.Printf("[VNC] New connection from %s, forwarding to %s", conn.RemoteAddr(), vncAddr)

	// 通过隧道连接到客户端本地的 VNC 服务
	remote, err := dialer.Dial("tcp", vncAddr)
	if err != nil {
		log.Printf("[VNC] Failed to connect to %s: %v", vncAddr, err)
		return err
	}
	defer remote.Close()

	// 双向转发 VNC 流量
	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(remote, conn)
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(conn, remote)
		errCh <- err
	}()

	// 等待任一方向完成
	<-errCh
	return nil
}

// Close 释放资源
func (p *VNCPlugin) Close() error {
	return nil
}
