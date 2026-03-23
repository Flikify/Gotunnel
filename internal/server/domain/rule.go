package domain

// ProxyRule is the server-side rule model used by storage and runtime.
type ProxyRule struct {
	Name         string `json:"name" yaml:"name"`
	Type         string `json:"type" yaml:"type"`
	LocalIP      string `json:"local_ip" yaml:"local_ip"`
	LocalPort    int    `json:"local_port" yaml:"local_port"`
	RemotePort   int    `json:"remote_port" yaml:"remote_port"`
	Enabled      *bool  `json:"enabled,omitempty" yaml:"enabled"`
	AuthEnabled  bool   `json:"auth_enabled,omitempty" yaml:"auth_enabled"`
	AuthUsername string `json:"auth_username,omitempty" yaml:"auth_username"`
	AuthPassword string `json:"auth_password,omitempty" yaml:"auth_password"`
	PortStatus   string `json:"port_status,omitempty" yaml:"-"`
}

// IsEnabled checks whether the rule is enabled. Nil defaults to true.
func (r *ProxyRule) IsEnabled() bool {
	if r.Enabled == nil {
		return true
	}
	return *r.Enabled
}
