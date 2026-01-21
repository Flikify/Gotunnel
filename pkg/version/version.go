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
	RepoURL    = "https://git.92coco.cn/flik/GoTunnel"
	APIBaseURL = "https://git.92coco.cn/api/v1"
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
// Gitea 兼容：先尝试 /releases/latest，失败则尝试 /releases 取第一个
func GetLatestRelease() (*ReleaseInfo, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// 首先尝试 /releases/latest 端点（GitHub 兼容）
	latestURL := fmt.Sprintf("%s/repos/%s/%s/releases/latest", APIBaseURL, RepoOwner, RepoName)
	resp, err := client.Get(latestURL)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var release ReleaseInfo
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		return &release, nil
	}

	// 如果 /releases/latest 不可用，尝试 /releases 并取第一个
	resp.Body.Close()
	listURL := fmt.Sprintf("%s/repos/%s/%s/releases?limit=1", APIBaseURL, RepoOwner, RepoName)
	resp, err = client.Get(listURL)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var releases []ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found in repository")
	}

	return &releases[0], nil
}

// CheckUpdate 检查更新（返回最新版本信息）
func CheckUpdate(component string) (*UpdateInfo, error) {
	release, err := GetLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("get latest release: %w", err)
	}

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
	var downloadURL string
	var assetName string
	var assetSize int64

	if asset := findAssetForPlatform(release.Assets, component, osName, arch); asset != nil {
		downloadURL = asset.BrowserDownloadURL
		assetName = asset.Name
		assetSize = asset.Size
	}

	return &UpdateInfo{
		Latest:      release.TagName,
		ReleaseNote: release.Body,
		DownloadURL: downloadURL,
		AssetName:   assetName,
		AssetSize:   assetSize,
	}, nil
}

// findAssetForPlatform 在 Release 资产中查找匹配的文件
func findAssetForPlatform(assets []ReleaseAsset, component, osName, arch string) *ReleaseAsset {
	// 构建匹配模式
	// CI 格式: gotunnel-server-v1.0.0-linux-amd64.tar.gz
	// 或者:    gotunnel-client-v1.0.0-windows-amd64.zip
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
