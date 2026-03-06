package handler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gotunnel/pkg/update"
	"github.com/gotunnel/pkg/version"
)

// UpdateInfo 更新信息
type UpdateInfo struct {
	Available   bool   `json:"available"`
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	ReleaseNote string `json:"release_note"`
	DownloadURL string `json:"download_url"`
	AssetName   string `json:"asset_name"`
	AssetSize   int64  `json:"asset_size"`
}

// checkUpdateForComponent 检查组件更新
func checkUpdateForComponent(component string) (*UpdateInfo, error) {
	release, err := version.GetLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("get latest release: %w", err)
	}

	latestVersion := release.TagName
	currentVersion := version.Version
	available := version.CompareVersions(currentVersion, latestVersion) < 0

	// 查找对应平台的资产
	var downloadURL string
	var assetName string
	var assetSize int64

	if asset := findAssetForPlatform(release.Assets, component, runtime.GOOS, runtime.GOARCH); asset != nil {
		downloadURL = asset.BrowserDownloadURL
		assetName = asset.Name
		assetSize = asset.Size
	}

	return &UpdateInfo{
		Available:   available,
		Current:     currentVersion,
		Latest:      latestVersion,
		ReleaseNote: release.Body,
		DownloadURL: downloadURL,
		AssetName:   assetName,
		AssetSize:   assetSize,
	}, nil
}

// checkClientUpdateForPlatform 检查指定平台的客户端更新
func checkClientUpdateForPlatform(osName, arch string) (*UpdateInfo, error) {
	if osName == "" {
		osName = runtime.GOOS
	}
	if arch == "" {
		arch = runtime.GOARCH
	}

	release, err := version.GetLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("get latest release: %w", err)
	}

	latestVersion := release.TagName

	// 查找对应平台的资产
	var downloadURL string
	var assetName string
	var assetSize int64

	if asset := findAssetForPlatform(release.Assets, "client", osName, arch); asset != nil {
		downloadURL = asset.BrowserDownloadURL
		assetName = asset.Name
		assetSize = asset.Size
	}

	return &UpdateInfo{
		Available:   true,
		Current:     "",
		Latest:      latestVersion,
		ReleaseNote: release.Body,
		DownloadURL: downloadURL,
		AssetName:   assetName,
		AssetSize:   assetSize,
	}, nil
}

// findAssetForPlatform 在 Release 资产中查找匹配的文件
// CI 格式: gotunnel-server-v1.0.0-linux-amd64.tar.gz
// 或者:    gotunnel-client-v1.0.0-windows-amd64.zip
func findAssetForPlatform(assets []version.ReleaseAsset, component, osName, arch string) *version.ReleaseAsset {
	prefix := fmt.Sprintf("gotunnel-%s-", component)
	suffix := fmt.Sprintf("-%s-%s", osName, arch)

	for i := range assets {
		name := assets[i].Name
		// 检查是否匹配 gotunnel-{component}-{version}-{os}-{arch}.{ext}
		if strings.HasPrefix(name, prefix) && strings.Contains(name, suffix) {
			return &assets[i]
		}
	}
	return nil
}

// performSelfUpdate 执行自更新
func performSelfUpdate(downloadURL string, restart bool) error {
	// 使用共享的下载和解压逻辑
	binaryPath, cleanup, err := update.DownloadAndExtract(downloadURL, "server")
	if err != nil {
		return err
	}
	defer cleanup()

	// 获取当前可执行文件路径
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}
	currentPath, _ = filepath.EvalSymlinks(currentPath)

	// Windows 需要特殊处理（运行中的文件无法直接替换）
	if runtime.GOOS == "windows" {
		return performWindowsUpdate(binaryPath, currentPath, restart)
	}

	// Linux/Mac: 直接替换
	backupPath := currentPath + ".bak"

	// 备份当前文件
	if err := os.Rename(currentPath, backupPath); err != nil {
		return fmt.Errorf("backup current: %w", err)
	}

	// 复制新文件（不能用 rename，可能跨文件系统）
	if err := update.CopyFile(binaryPath, currentPath); err != nil {
		os.Rename(backupPath, currentPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	// 设置执行权限
	if err := os.Chmod(currentPath, 0755); err != nil {
		os.Rename(backupPath, currentPath)
		return fmt.Errorf("chmod new binary: %w", err)
	}

	// 删除备份
	os.Remove(backupPath)

	if restart {
		restartProcess(currentPath)
	}

	return nil
}

// performWindowsUpdate Windows 平台更新
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

// restartProcess 重启进程
func restartProcess(path string) {
	cmd := exec.Command(path, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
	os.Exit(0)
}
