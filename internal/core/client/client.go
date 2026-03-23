package client

import corerule "github.com/gotunnel/internal/core/rule"

// Client is the shared client aggregate used by storage, runtime, and application services.
type Client struct {
	ID             string               `json:"id"`
	Nickname       string               `json:"nickname,omitempty"`
	Rules          []corerule.ProxyRule `json:"rules"`
	LastRemoteAddr string               `json:"last_remote_addr,omitempty"`
	LastOS         string               `json:"last_os,omitempty"`
	LastArch       string               `json:"last_arch,omitempty"`
	LastVersion    string               `json:"last_version,omitempty"`
	LastOfflineAt  int64                `json:"last_offline_at,omitempty"`
}
