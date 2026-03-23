package domain

// Client is the server-side client aggregate used by storage and services.
type Client struct {
	ID             string      `json:"id"`
	Nickname       string      `json:"nickname,omitempty"`
	Rules          []ProxyRule `json:"rules"`
	LastRemoteAddr string      `json:"last_remote_addr,omitempty"`
	LastOS         string      `json:"last_os,omitempty"`
	LastArch       string      `json:"last_arch,omitempty"`
	LastVersion    string      `json:"last_version,omitempty"`
	LastOfflineAt  int64       `json:"last_offline_at,omitempty"`
}
