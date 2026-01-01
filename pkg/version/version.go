package version

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// 版本信息
const Version = "1.0.0"

// 仓库信息
const (
	RepoURL    = "https://git.92coco.cn:8443/flik/GoTunnel"
	APIBaseURL = "https://git.92coco.cn:8443/api/v1"
	RepoOwner  = "flik"
	RepoName   = "GoTunnel"
)

// Info 版本详细信息
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// GetInfo 获取版本信息
func GetInfo() Info {
	return Info{
		Version:   Version,
		GitCommit: "",
		BuildTime: "",
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// ReleaseInfo Release 信息
type ReleaseInfo struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	Body        string         `json:"body"`
	PublishedAt string         `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`
}

// ReleaseAsset Release 资产
type ReleaseAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// UpdateInfo 更新信息
type UpdateInfo struct {
	Latest      string `json:"latest"`
	ReleaseNote string `json:"release_note"`
	DownloadURL string `json:"download_url"`
	AssetName   string `json:"asset_name"`
	AssetSize   int64  `json:"asset_size"`
}

// GetLatestRelease 获取最新 Release
func GetLatestRelease() (*ReleaseInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", APIBaseURL, RepoOwner, RepoName)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// CheckUpdate 检查更新（返回最新版本信息）
func CheckUpdate(component string) (*UpdateInfo, error) {
	release, err := GetLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("get latest release: %w", err)
	}

	// 查找对应平台的资产
	assetName := getAssetName(component)
	var downloadURL string
	var assetSize int64

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			assetSize = asset.Size
			break
		}
	}

	return &UpdateInfo{
		Latest:      release.TagName,
		ReleaseNote: release.Body,
		DownloadURL: downloadURL,
		AssetName:   assetName,
		AssetSize:   assetSize,
	}, nil
}

// CheckUpdateForPlatform 检查指定平台的更新
func CheckUpdateForPlatform(component, osName, arch string) (*UpdateInfo, error) {
	release, err := GetLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("get latest release: %w", err)
	}

	// 查找对应平台的资产
	assetName := getAssetNameForPlatform(component, osName, arch)
	var downloadURL string
	var assetSize int64

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			assetSize = asset.Size
			break
		}
	}

	return &UpdateInfo{
		Latest:      release.TagName,
		ReleaseNote: release.Body,
		DownloadURL: downloadURL,
		AssetName:   assetName,
		AssetSize:   assetSize,
	}, nil
}

// getAssetName 获取当前平台的资产文件名
func getAssetName(component string) string {
	return getAssetNameForPlatform(component, runtime.GOOS, runtime.GOARCH)
}

// getAssetNameForPlatform 获取指定平台的资产文件名
func getAssetNameForPlatform(component, osName, arch string) string {
	ext := ""
	if osName == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("%s_%s_%s%s", component, osName, arch, ext)
}

// CompareVersions 比较版本号
// 返回: -1 (v1 < v2), 0 (v1 == v2), 1 (v1 > v2)
func CompareVersions(v1, v2 string) int {
	parts1 := parseVersionParts(v1)
	parts2 := parseVersionParts(v2)

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int
		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}

		if p1 < p2 {
			return -1
		}
		if p1 > p2 {
			return 1
		}
	}
	return 0
}

func parseVersionParts(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	result := make([]int, len(parts))
	for i, p := range parts {
		n, _ := strconv.Atoi(p)
		result[i] = n
	}
	return result
}
