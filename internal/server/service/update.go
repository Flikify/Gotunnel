package service

import "github.com/gotunnel/internal/server/updateapp"

type updateRuntime interface {
	SendUpdateToClient(clientID, downloadURL string) error
}

type updateConfig interface {
	Snapshot() interface{ Server struct{ Web struct{ CDNPrefix string } } }
}

type UpdateService interface {
	CheckServer() (*updateapp.Info, error)
	CheckClient(osName, arch string) (*updateapp.Info, error)
	GetServerUpdateStatus() (*updateapp.ServerUpdateStatus, error)
	ApplyServer(downloadURL, targetVersion string, restart bool) error
	ApplyClient(clientID, downloadURL string) error
}

type updateService struct {
	runtime updateRuntime
	config  updateConfig
}

func NewUpdateService(runtime updateRuntime, config updateConfig) UpdateService {
	return &updateService{runtime: runtime, config: config}
}

func (s *updateService) CheckServer() (*updateapp.Info, error) {
	cdnPrefix := ""
	if s.config != nil {
		cdnPrefix = s.config.Snapshot().Server.Web.CDNPrefix
	}
	return updateapp.CheckForComponentWithCDN("server", cdnPrefix)
}

func (s *updateService) CheckClient(osName, arch string) (*updateapp.Info, error) {
	cdnPrefix := ""
	if s.config != nil {
		cdnPrefix = s.config.Snapshot().Server.Web.CDNPrefix
	}
	return updateapp.CheckClientForPlatformWithCDN(osName, arch, cdnPrefix)
}

func (s *updateService) GetServerUpdateStatus() (*updateapp.ServerUpdateStatus, error) {
	return updateapp.GetServerUpdateStatus()
}

func (s *updateService) ApplyServer(downloadURL, targetVersion string, restart bool) error {
	if err := updateapp.BeginServerUpdate(targetVersion); err != nil {
		return err
	}

	go func() {
		if err := updateapp.PerformSelfUpdate(downloadURL, targetVersion, restart); err != nil {
			_ = updateapp.MarkServerUpdateFailed(targetVersion, err.Error())
		}
	}()

	return nil
}

func (s *updateService) ApplyClient(clientID, downloadURL string) error {
	return s.runtime.SendUpdateToClient(clientID, downloadURL)
}
