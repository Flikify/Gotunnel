package service

import "github.com/gotunnel/internal/server/updateapp"

type updateRuntime interface {
	SendUpdateToClient(clientID, downloadURL string) error
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
}

func NewUpdateService(runtime updateRuntime) UpdateService {
	return &updateService{runtime: runtime}
}

func (s *updateService) CheckServer() (*updateapp.Info, error) {
	return updateapp.CheckForComponent("server")
}

func (s *updateService) CheckClient(osName, arch string) (*updateapp.Info, error) {
	return updateapp.CheckClientForPlatform(osName, arch)
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
