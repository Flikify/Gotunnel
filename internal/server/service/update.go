package service

import "github.com/gotunnel/internal/server/updateapp"

type updateRuntime interface {
	SendUpdateToClient(clientID, downloadURL string) error
}

type UpdateService interface {
	CheckServer() (*updateapp.Info, error)
	CheckClient(osName, arch string) (*updateapp.Info, error)
	ApplyServer(downloadURL string, restart bool) error
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

func (s *updateService) ApplyServer(downloadURL string, restart bool) error {
	return updateapp.PerformSelfUpdate(downloadURL, restart)
}

func (s *updateService) ApplyClient(clientID, downloadURL string) error {
	return s.runtime.SendUpdateToClient(clientID, downloadURL)
}
