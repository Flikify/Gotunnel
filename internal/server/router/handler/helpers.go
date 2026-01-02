package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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
	// 下载新版本
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "gotunnel_update_"+time.Now().Format("20060102150405"))

	if runtime.GOOS == "windows" {
		tempFile += ".exe"
	}

	if err := downloadFile(downloadURL, tempFile); err != nil {
		return fmt.Errorf("download update: %w", err)
	}

	// 设置执行权限
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tempFile, 0755); err != nil {
			os.Remove(tempFile)
			return fmt.Errorf("chmod: %w", err)
		}
	}

	// 获取当前可执行文件路径
	currentPath, err := os.Executable()
	if err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("get executable: %w", err)
	}
	currentPath, _ = filepath.EvalSymlinks(currentPath)

	// Windows 需要特殊处理（运行中的文件无法直接替换）
	if runtime.GOOS == "windows" {
		return performWindowsUpdate(tempFile, currentPath, restart)
	}

	// Linux/Mac: 直接替换
	backupPath := currentPath + ".bak"

	// 备份当前文件
	if err := os.Rename(currentPath, backupPath); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("backup current: %w", err)
	}

	// 移动新文件
	if err := os.Rename(tempFile, currentPath); err != nil {
		os.Rename(backupPath, currentPath)
		return fmt.Errorf("replace binary: %w", err)
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

// downloadFile 下载文件
func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
