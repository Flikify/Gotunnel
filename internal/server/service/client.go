package service

import (
	"fmt"

	"github.com/gotunnel/internal/server/domain"
)

type ClientRepository interface {
	GetAllClients() ([]domain.Client, error)
	GetClient(id string) (*domain.Client, error)
	CreateClient(c *domain.Client) error
	UpdateClient(c *domain.Client) error
	DeleteClient(id string) error
	ClientExists(id string) (bool, error)
	GetClientRules(id string) ([]domain.ProxyRule, error)
}

type ClientRuntimeStatus struct {
	Online     bool
	LastPing   string
	RemoteAddr string
	Name       string
	OS         string
	Arch       string
	Version    string
}

type ClientRuntime interface {
	IsClientOnline(clientID string) bool
	GetClientStatus(clientID string) (ClientRuntimeStatus, bool)
	GetAllClientStatus() map[string]ClientRuntimeStatus
	PushConfigToClient(clientID string) error
	DisconnectClient(clientID string) error
	RestartClient(clientID string) error
}

type clientRuntimeSource interface {
	IsClientOnline(clientID string) bool
	GetClientStatus(clientID string) (online bool, lastPing, remoteAddr, clientName, clientOS, clientArch, clientVersion string)
	GetAllClientStatus() map[string]struct {
		Online     bool
		LastPing   string
		RemoteAddr string
		Name       string
		OS         string
		Arch       string
		Version    string
	}
	PushConfigToClient(clientID string) error
	DisconnectClient(clientID string) error
	RestartClient(clientID string) error
}

type clientRuntimeAdapter struct {
	source clientRuntimeSource
}

func NewClientRuntimeAdapter(source clientRuntimeSource) ClientRuntime {
	return &clientRuntimeAdapter{source: source}
}

func (a *clientRuntimeAdapter) IsClientOnline(clientID string) bool {
	return a.source.IsClientOnline(clientID)
}

func (a *clientRuntimeAdapter) GetClientStatus(clientID string) (ClientRuntimeStatus, bool) {
	online, lastPing, remoteAddr, name, osName, arch, version := a.source.GetClientStatus(clientID)
	return ClientRuntimeStatus{
		Online:     online,
		LastPing:   lastPing,
		RemoteAddr: remoteAddr,
		Name:       name,
		OS:         osName,
		Arch:       arch,
		Version:    version,
	}, online
}

func (a *clientRuntimeAdapter) GetAllClientStatus() map[string]ClientRuntimeStatus {
	statuses := a.source.GetAllClientStatus()
	result := make(map[string]ClientRuntimeStatus, len(statuses))
	for clientID, status := range statuses {
		result[clientID] = ClientRuntimeStatus{
			Online:     status.Online,
			LastPing:   status.LastPing,
			RemoteAddr: status.RemoteAddr,
			Name:       status.Name,
			OS:         status.OS,
			Arch:       status.Arch,
			Version:    status.Version,
		}
	}
	return result
}

func (a *clientRuntimeAdapter) PushConfigToClient(clientID string) error {
	return a.source.PushConfigToClient(clientID)
}

func (a *clientRuntimeAdapter) DisconnectClient(clientID string) error {
	return a.source.DisconnectClient(clientID)
}

func (a *clientRuntimeAdapter) RestartClient(clientID string) error {
	return a.source.RestartClient(clientID)
}

type ClientService interface {
	ListClients() ([]ClientListItem, error)
	CreateClient(input CreateClientInput) error
	GetClient(id string) (*ClientDetail, error)
	UpdateClient(id string, input UpdateClientInput) error
	DeleteClient(id string) error
	PushConfig(clientID string) error
	DisconnectClient(clientID string) error
	RestartClient(clientID string) error
}

type CreateClientInput struct {
	ID    string
	Rules []domain.ProxyRule
}

type UpdateClientInput struct {
	Nickname string
	Rules    []domain.ProxyRule
}

type ClientListItem struct {
	ID            string
	Nickname      string
	Online        bool
	LastPing      string
	LastOfflineAt int64
	RemoteAddr    string
	RuleCount     int
	OS            string
	Arch          string
	Version       string
}

type ClientDetail struct {
	ID            string
	Nickname      string
	Rules         []domain.ProxyRule
	Online        bool
	LastPing      string
	LastOfflineAt int64
	RemoteAddr    string
	OS            string
	Arch          string
	Version       string
}

type clientService struct {
	repo    ClientRepository
	runtime ClientRuntime
	config  ConfigService
}

func NewClientService(repo ClientRepository, runtime ClientRuntime, config ConfigService) ClientService {
	return &clientService{
		repo:    repo,
		runtime: runtime,
		config:  config,
	}
}

func (s *clientService) ListClients() ([]ClientListItem, error) {
	clients, err := s.repo.GetAllClients()
	if err != nil {
		return nil, err
	}

	statusMap := s.runtime.GetAllClientStatus()
	result := make([]ClientListItem, 0, len(clients))
	for _, client := range clients {
		item := ClientListItem{
			ID:            client.ID,
			Nickname:      client.Nickname,
			LastOfflineAt: client.LastOfflineAt,
			RemoteAddr:    client.LastRemoteAddr,
			RuleCount:     len(client.Rules),
			OS:            client.LastOS,
			Arch:          client.LastArch,
			Version:       client.LastVersion,
		}
		if status, ok := statusMap[client.ID]; ok {
			item.Online = status.Online
			item.LastPing = status.LastPing
			item.RemoteAddr = status.RemoteAddr
			item.OS = status.OS
			item.Arch = status.Arch
			item.Version = status.Version
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *clientService) CreateClient(input CreateClientInput) error {
	if !validateClientID(input.ID) {
		return ErrInvalidClientID
	}
	if err := s.validateProxyRuleLimit(input.Rules); err != nil {
		return err
	}

	exists, err := s.repo.ClientExists(input.ID)
	if err != nil {
		return err
	}
	if exists {
		return ErrClientAlreadyExists
	}

	return s.repo.CreateClient(&domain.Client{
		ID:    input.ID,
		Rules: cloneRules(input.Rules),
	})
}

func (s *clientService) GetClient(id string) (*ClientDetail, error) {
	client, err := s.repo.GetClient(id)
	if err != nil {
		return nil, ErrClientNotFound
	}

	status, online := s.runtime.GetClientStatus(id)
	nickname := client.Nickname
	if online && status.Name != "" && nickname == "" {
		nickname = status.Name
	}

	detail := &ClientDetail{
		ID:            client.ID,
		Nickname:      nickname,
		Rules:         cloneRules(client.Rules),
		Online:        online,
		LastPing:      status.LastPing,
		LastOfflineAt: client.LastOfflineAt,
		RemoteAddr:    status.RemoteAddr,
		OS:            status.OS,
		Arch:          status.Arch,
		Version:       status.Version,
	}
	if !online {
		detail.RemoteAddr = client.LastRemoteAddr
		detail.OS = client.LastOS
		detail.Arch = client.LastArch
		detail.Version = client.LastVersion
	}
	return detail, nil
}

func (s *clientService) UpdateClient(id string, input UpdateClientInput) error {
	if err := s.validateProxyRuleLimit(input.Rules); err != nil {
		return err
	}

	client, err := s.repo.GetClient(id)
	if err != nil {
		return ErrClientNotFound
	}

	client.Nickname = input.Nickname
	client.Rules = cloneRules(input.Rules)
	return s.repo.UpdateClient(client)
}

func (s *clientService) DeleteClient(id string) error {
	exists, err := s.repo.ClientExists(id)
	if err != nil {
		return err
	}
	if !exists {
		return ErrClientNotFound
	}
	return s.repo.DeleteClient(id)
}

func (s *clientService) PushConfig(clientID string) error {
	if !s.runtime.IsClientOnline(clientID) {
		return ErrClientNotOnline
	}
	return s.runtime.PushConfigToClient(clientID)
}

func (s *clientService) DisconnectClient(clientID string) error {
	return s.runtime.DisconnectClient(clientID)
}

func (s *clientService) RestartClient(clientID string) error {
	return s.runtime.RestartClient(clientID)
}

func (s *clientService) validateProxyRuleLimit(rules []domain.ProxyRule) error {
	limit := s.config.MaxClientProxies()
	if limit <= 0 || len(rules) <= limit {
		return nil
	}
	return fmt.Errorf("%w: at most %d proxies are allowed per client", ErrProxyRuleLimitExceeded, limit)
}

func cloneRules(rules []domain.ProxyRule) []domain.ProxyRule {
	if len(rules) == 0 {
		return nil
	}
	cloned := make([]domain.ProxyRule, len(rules))
	copy(cloned, rules)
	return cloned
}

func validateClientID(id string) bool {
	if len(id) < 1 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
