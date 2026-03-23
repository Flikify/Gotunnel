package service

import (
	"sort"
	"sync"

	"github.com/gotunnel/internal/server/config"
)

// ConfigRuntime applies runtime configuration to the tunnel server.
type ConfigRuntime interface {
	ApplyRuntimeConfig(heartbeatSec, heartbeatTimeoutSec, maxClientProxies, clientResponseTimeoutSec int)
}

// ServerConfigUpdate describes the mutable server settings.
type ServerConfigUpdate struct {
	BindAddr                 string
	BindPort                 int
	Token                    string
	HeartbeatSec             *int
	HeartbeatTimeout         *int
	MaxClientProxies         *int
	ClientResponseTimeoutSec *int
}

// WebConfigUpdate describes the mutable web settings.
type WebConfigUpdate struct {
	Enabled  *bool
	BindPort *int
	Username *string
	Password *string
}

// ConfigUpdate is the aggregate config change request.
type ConfigUpdate struct {
	Server *ServerConfigUpdate
	Web    *WebConfigUpdate
}

// ConfigUpdateResult reports which changes were applied immediately and which require restart.
type ConfigUpdateResult struct {
	AppliedRuntimeFields  []string
	RestartRequiredFields []string
}

// PersistConfigResult reports which persisted fields can be applied to runtime and which require restart.
type PersistConfigResult struct {
	RuntimeApplyFields    []string
	RestartRequiredFields []string
}

// ConfigService manages persisted server configuration and runtime updates.
type ConfigService interface {
	Snapshot() config.ServerConfig
	Persist(update ConfigUpdate) (PersistConfigResult, error)
	ApplyRuntimeConfig(fields []string) ConfigUpdateResult
	MaxClientProxies() int
}

type configService struct {
	mu         sync.RWMutex
	cfg        *config.ServerConfig
	configPath string
	runtime    ConfigRuntime
}

// NewConfigService creates a config service backed by the shared server config.
func NewConfigService(cfg *config.ServerConfig, configPath string, runtime ConfigRuntime) ConfigService {
	return &configService{
		cfg:        cfg,
		configPath: configPath,
		runtime:    runtime,
	}
}

func (s *configService) Snapshot() config.ServerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return *s.cfg
}

func (s *configService) MaxClientProxies() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.cfg.Server.MaxClientProxies
}

func (s *configService) Persist(update ConfigUpdate) (PersistConfigResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := *s.cfg
	appliedRuntimeFields := map[string]struct{}{}
	restartRequiredFields := map[string]struct{}{}

	if update.Server != nil {
		heartbeatSec := next.Server.HeartbeatSec
		if update.Server.HeartbeatSec != nil {
			heartbeatSec = *update.Server.HeartbeatSec
		}
		heartbeatTimeout := next.Server.HeartbeatTimeout
		if update.Server.HeartbeatTimeout != nil {
			heartbeatTimeout = *update.Server.HeartbeatTimeout
		}
		if heartbeatTimeout < heartbeatSec {
			return PersistConfigResult{}, ErrInvalidHeartbeatConfig
		}

		if update.Server.BindAddr != "" && update.Server.BindAddr != next.Server.BindAddr {
			next.Server.BindAddr = update.Server.BindAddr
			restartRequiredFields["bind_addr"] = struct{}{}
		}
		if update.Server.BindPort > 0 && update.Server.BindPort != next.Server.BindPort {
			next.Server.BindPort = update.Server.BindPort
			restartRequiredFields["bind_port"] = struct{}{}
		}
		if update.Server.Token != "" && update.Server.Token != next.Server.Token {
			next.Server.Token = update.Server.Token
			restartRequiredFields["token"] = struct{}{}
		}
		if update.Server.HeartbeatSec != nil && *update.Server.HeartbeatSec != next.Server.HeartbeatSec {
			next.Server.HeartbeatSec = *update.Server.HeartbeatSec
			appliedRuntimeFields["heartbeat_sec"] = struct{}{}
		}
		if update.Server.HeartbeatTimeout != nil && *update.Server.HeartbeatTimeout != next.Server.HeartbeatTimeout {
			next.Server.HeartbeatTimeout = *update.Server.HeartbeatTimeout
			appliedRuntimeFields["heartbeat_timeout"] = struct{}{}
		}
		if update.Server.MaxClientProxies != nil && *update.Server.MaxClientProxies != next.Server.MaxClientProxies {
			next.Server.MaxClientProxies = *update.Server.MaxClientProxies
			appliedRuntimeFields["max_client_proxies"] = struct{}{}
		}
		if update.Server.ClientResponseTimeoutSec != nil && *update.Server.ClientResponseTimeoutSec != next.Server.ClientResponseTimeoutSec {
			next.Server.ClientResponseTimeoutSec = *update.Server.ClientResponseTimeoutSec
			appliedRuntimeFields["client_response_timeout_sec"] = struct{}{}
		}
	}

	if update.Web != nil {
		if update.Web.Enabled != nil && *update.Web.Enabled != next.Server.Web.Enabled {
			next.Server.Web.Enabled = *update.Web.Enabled
			restartRequiredFields["web.enabled"] = struct{}{}
		}
		if update.Web.BindPort != nil && *update.Web.BindPort > 0 && *update.Web.BindPort != next.Server.Web.BindPort {
			next.Server.Web.BindPort = *update.Web.BindPort
			restartRequiredFields["web.bind_port"] = struct{}{}
		}
		if update.Web.Username != nil && *update.Web.Username != next.Server.Web.Username {
			next.Server.Web.Username = *update.Web.Username
			restartRequiredFields["web.username"] = struct{}{}
		}
		if update.Web.Password != nil && *update.Web.Password != next.Server.Web.Password {
			next.Server.Web.Password = *update.Web.Password
			restartRequiredFields["web.password"] = struct{}{}
		}
	}

	if err := config.SaveServerConfig(s.configPath, &next); err != nil {
		return PersistConfigResult{}, err
	}

	*s.cfg = next
	result := PersistConfigResult{
		RuntimeApplyFields:    sortedFieldNames(appliedRuntimeFields),
		RestartRequiredFields: sortedFieldNames(restartRequiredFields),
	}
	return result, nil
}

func (s *configService) ApplyRuntimeConfig(fields []string) ConfigUpdateResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(fields) == 0 {
		return ConfigUpdateResult{}
	}

	if s.runtime != nil {
		s.runtime.ApplyRuntimeConfig(
			s.cfg.Server.HeartbeatSec,
			s.cfg.Server.HeartbeatTimeout,
			s.cfg.Server.MaxClientProxies,
			s.cfg.Server.ClientResponseTimeoutSec,
		)
	}

	return ConfigUpdateResult{
		AppliedRuntimeFields: append([]string(nil), fields...),
	}
}

func sortedFieldNames(fields map[string]struct{}) []string {
	if len(fields) == 0 {
		return nil
	}

	names := make([]string, 0, len(fields))
	for field := range fields {
		names = append(names, field)
	}
	sort.Strings(names)
	return names
}
