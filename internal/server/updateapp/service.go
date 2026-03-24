package updateapp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	sharedupdate "github.com/gotunnel/pkg/update"
	"github.com/gotunnel/pkg/version"
)

// Info describes the current update state returned to the web layer.
type Info struct {
	Available   bool   `json:"available"`
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	ReleaseNote string `json:"release_note"`
	DownloadURL string `json:"download_url"`
	AssetName   string `json:"asset_name"`
	AssetSize   int64  `json:"asset_size"`
}

// CheckForComponent returns update information for the running platform.
func CheckForComponent(component string) (*Info, error) {
	release, err := version.GetLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("get latest release: %w", err)
	}

	latestVersion := release.TagName
	currentVersion := version.Version
	updateInfo, err := version.CheckUpdate(component)
	if err == nil && updateInfo != nil {
		return &Info{
			Available:   version.CompareVersions(currentVersion, latestVersion) < 0,
			Current:     currentVersion,
			Latest:      latestVersion,
			ReleaseNote: release.Body,
			DownloadURL: updateInfo.DownloadURL,
			AssetName:   updateInfo.AssetName,
			AssetSize:   updateInfo.AssetSize,
		}, nil
	}

	return &Info{
		Available:   version.CompareVersions(currentVersion, latestVersion) < 0,
		Current:     currentVersion,
		Latest:      latestVersion,
		ReleaseNote: release.Body,
	}, nil
}

// CheckClientForPlatform returns the matching client package for a target platform.
func CheckClientForPlatform(osName, arch string) (*Info, error) {
	if osName == "" {
		osName = runtime.GOOS
	}
	if arch == "" {
		arch = runtime.GOARCH
	}

	updateInfo, err := version.CheckUpdateForPlatform("client", osName, arch)
	if err != nil {
		return nil, fmt.Errorf("get latest release: %w", err)
	}

	return &Info{
		Available:   true,
		Current:     "",
		Latest:      updateInfo.Latest,
		ReleaseNote: updateInfo.ReleaseNote,
		DownloadURL: updateInfo.DownloadURL,
		AssetName:   updateInfo.AssetName,
		AssetSize:   updateInfo.AssetSize,
	}, nil
}

// PerformSelfUpdate downloads and swaps in a new server binary.
func PerformSelfUpdate(downloadURL, targetVersion string, restart bool) error {
	fail := func(err error) error {
		_ = MarkServerUpdateFailed(targetVersion, err.Error())
		return err
	}

	if err := MarkServerUpdateApplying(targetVersion); err != nil {
		return err
	}

	binaryPath, cleanup, err := sharedupdate.DownloadAndExtract(downloadURL, "server")
	if err != nil {
		return fail(err)
	}
	defer cleanup()

	currentPath, err := os.Executable()
	if err != nil {
		return fail(fmt.Errorf("get executable: %w", err))
	}
	currentPath, _ = filepath.EvalSymlinks(currentPath)

	if runtime.GOOS == "windows" {
		if err := MarkServerUpdateRestarting(targetVersion); err != nil {
			return err
		}
		return performWindowsUpdate(binaryPath, currentPath, restart)
	}

	backupPath := currentPath + ".bak"
	if err := os.Rename(currentPath, backupPath); err != nil {
		return fail(fmt.Errorf("backup current: %w", err))
	}

	if err := sharedupdate.CopyFile(binaryPath, currentPath); err != nil {
		_ = os.Rename(backupPath, currentPath)
		return fail(fmt.Errorf("replace binary: %w", err))
	}

	if err := os.Chmod(currentPath, 0755); err != nil {
		_ = os.Rename(backupPath, currentPath)
		return fail(fmt.Errorf("chmod new binary: %w", err))
	}

	_ = os.Remove(backupPath)

	if restart {
		if err := MarkServerUpdateRestarting(targetVersion); err != nil {
			return err
		}
		restartProcess(currentPath)
	}

	if err := MarkServerUpdateSucceeded(targetVersion, "新版本已写入，重启服务后生效"); err != nil {
		return err
	}

	return nil
}

func performWindowsUpdate(newFile, currentPath string, restart bool) error {
	batchScript := fmt.Sprintf(`@echo off
:: Check for admin rights, request UAC elevation if needed
net session >nul 2>&1
if %%errorlevel%% neq 0 (
    powershell -Command "Start-Process cmd -ArgumentList '/C \\"\"%%~f0\"\"' -Verb RunAs"
    exit /b
)
ping 127.0.0.1 -n 2 > nul
del "%s"
move "%s" "%s"
`, currentPath, newFile, currentPath)

	if restart {
		batchScript += fmt.Sprintf(`start "" "%s"
`, currentPath)
	}

	batchScript += "del \"%~f0\"\n"

	batchPath := filepath.Join(os.TempDir(), "gotunnel_update.bat")
	if err := os.WriteFile(batchPath, []byte(batchScript), 0755); err != nil {
		return fmt.Errorf("write batch: %w", err)
	}

	cmd := exec.Command("cmd", "/C", "start", "/MIN", batchPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start batch: %w", err)
	}

	os.Exit(0)
	return nil
}

func restartProcess(path string) {
	cmd := exec.Command(path, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Start()
	os.Exit(0)
}
