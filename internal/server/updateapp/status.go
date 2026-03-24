package updateapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gotunnel/pkg/version"
)

var ErrUpdateInProgress = errors.New("server update already in progress")

const (
	ServerUpdateStateIdle       = "idle"
	ServerUpdateStateRunning    = "running"
	ServerUpdateStateRestarting = "restarting"
	ServerUpdateStateSucceeded  = "succeeded"
	ServerUpdateStateFailed     = "failed"
)

type ServerUpdateStatus struct {
	State          string `json:"state"`
	Message        string `json:"message,omitempty"`
	CurrentVersion string `json:"current_version,omitempty"`
	TargetVersion  string `json:"target_version,omitempty"`
	StartedAt      int64  `json:"started_at,omitempty"`
	FinishedAt     int64  `json:"finished_at,omitempty"`
	UpdatedAt      int64  `json:"updated_at"`
}

func (s ServerUpdateStatus) Active() bool {
	return s.State == ServerUpdateStateRunning || s.State == ServerUpdateStateRestarting
}

var (
	serverUpdateStatusPath = filepath.Join(os.TempDir(), "gotunnel-server-update-status.json")
	serverUpdateStatusMu   sync.Mutex
)

func GetServerUpdateStatus() (*ServerUpdateStatus, error) {
	serverUpdateStatusMu.Lock()
	defer serverUpdateStatusMu.Unlock()

	status, err := loadServerUpdateStatusLocked()
	if err != nil {
		return nil, err
	}

	normalized := normalizeServerUpdateStatus(status)
	if normalized != status {
		if err := saveServerUpdateStatusLocked(normalized); err != nil {
			return nil, err
		}
	}

	return &normalized, nil
}

func BeginServerUpdate(targetVersion string) error {
	return mutateServerUpdateStatus(func(status *ServerUpdateStatus) error {
		if status.Active() {
			return ErrUpdateInProgress
		}

		now := time.Now().Unix()
		*status = ServerUpdateStatus{
			State:          ServerUpdateStateRunning,
			Message:        "正在下载更新包",
			CurrentVersion: version.Version,
			TargetVersion:  targetVersion,
			StartedAt:      now,
			UpdatedAt:      now,
		}
		return nil
	})
}

func MarkServerUpdateApplying(targetVersion string) error {
	return mutateServerUpdateStatus(func(status *ServerUpdateStatus) error {
		now := time.Now().Unix()
		if status.StartedAt == 0 {
			status.StartedAt = now
		}
		status.State = ServerUpdateStateRunning
		status.Message = "正在安装新版本"
		status.CurrentVersion = version.Version
		status.TargetVersion = coalesceTargetVersion(targetVersion, status.TargetVersion)
		status.UpdatedAt = now
		status.FinishedAt = 0
		return nil
	})
}

func MarkServerUpdateRestarting(targetVersion string) error {
	return mutateServerUpdateStatus(func(status *ServerUpdateStatus) error {
		now := time.Now().Unix()
		if status.StartedAt == 0 {
			status.StartedAt = now
		}
		status.State = ServerUpdateStateRestarting
		status.Message = "更新已写入，正在重启服务"
		status.CurrentVersion = version.Version
		status.TargetVersion = coalesceTargetVersion(targetVersion, status.TargetVersion)
		status.UpdatedAt = now
		status.FinishedAt = 0
		return nil
	})
}

func MarkServerUpdateFailed(targetVersion, message string) error {
	return mutateServerUpdateStatus(func(status *ServerUpdateStatus) error {
		now := time.Now().Unix()
		if status.StartedAt == 0 {
			status.StartedAt = now
		}
		status.State = ServerUpdateStateFailed
		status.Message = message
		status.CurrentVersion = version.Version
		status.TargetVersion = coalesceTargetVersion(targetVersion, status.TargetVersion)
		status.UpdatedAt = now
		status.FinishedAt = now
		return nil
	})
}

func MarkServerUpdateSucceeded(targetVersion, message string) error {
	return mutateServerUpdateStatus(func(status *ServerUpdateStatus) error {
		now := time.Now().Unix()
		if status.StartedAt == 0 {
			status.StartedAt = now
		}
		status.State = ServerUpdateStateSucceeded
		status.Message = message
		status.CurrentVersion = version.Version
		status.TargetVersion = coalesceTargetVersion(targetVersion, status.TargetVersion)
		status.UpdatedAt = now
		status.FinishedAt = now
		return nil
	})
}

func mutateServerUpdateStatus(mutate func(*ServerUpdateStatus) error) error {
	serverUpdateStatusMu.Lock()
	defer serverUpdateStatusMu.Unlock()

	status, err := loadServerUpdateStatusLocked()
	if err != nil {
		return err
	}

	if err := mutate(&status); err != nil {
		return err
	}

	return saveServerUpdateStatusLocked(normalizeServerUpdateStatus(status))
}

func loadServerUpdateStatusLocked() (ServerUpdateStatus, error) {
	data, err := os.ReadFile(serverUpdateStatusPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultServerUpdateStatus(), nil
		}
		return ServerUpdateStatus{}, fmt.Errorf("read update status: %w", err)
	}

	var status ServerUpdateStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return ServerUpdateStatus{}, fmt.Errorf("decode update status: %w", err)
	}

	return status, nil
}

func saveServerUpdateStatusLocked(status ServerUpdateStatus) error {
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("encode update status: %w", err)
	}

	dir := filepath.Dir(serverUpdateStatusPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure update status dir: %w", err)
	}

	tempPath := serverUpdateStatusPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("write update status: %w", err)
	}

	if err := os.Rename(tempPath, serverUpdateStatusPath); err != nil {
		return fmt.Errorf("replace update status: %w", err)
	}

	return nil
}

func normalizeServerUpdateStatus(status ServerUpdateStatus) ServerUpdateStatus {
	now := time.Now().Unix()
	if status.State == "" {
		status = defaultServerUpdateStatus()
	}

	if status.CurrentVersion == "" {
		status.CurrentVersion = version.Version
	}

	if status.UpdatedAt == 0 {
		status.UpdatedAt = now
	}

	if status.Active() && status.TargetVersion != "" && version.CompareVersions(version.Version, status.TargetVersion) >= 0 {
		status.State = ServerUpdateStateSucceeded
		status.Message = "服务端已升级成功"
		status.CurrentVersion = version.Version
		if status.FinishedAt == 0 {
			status.FinishedAt = now
		}
		status.UpdatedAt = now
	}

	if status.State == ServerUpdateStateIdle || status.State == ServerUpdateStateSucceeded || status.State == ServerUpdateStateFailed {
		status.CurrentVersion = version.Version
	}

	return status
}

func defaultServerUpdateStatus() ServerUpdateStatus {
	return ServerUpdateStatus{
		State:          ServerUpdateStateIdle,
		CurrentVersion: version.Version,
		UpdatedAt:      time.Now().Unix(),
	}
}

func coalesceTargetVersion(next, current string) string {
	if next != "" {
		return next
	}
	return current
}
