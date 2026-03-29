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

var Version = "1.0.0"
var GitCommit = ""
var BuildTime = ""

const (
	RepoURL          = "https://github.com/Flikify/Gotunnel"
	APIBaseURL       = "https://api.github.com"
	RepoOwner        = "Flikify"
	RepoName         = "Gotunnel"
	GitHubAPIVersion = "2022-11-28"
	GitHubUserAgent  = "GoTunnel-Updater"
)

type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

type ReleaseInfo struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	Body        string         `json:"body"`
	PublishedAt string         `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`
}

type ReleaseAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type UpdateInfo struct {
	Latest      string `json:"latest"`
	ReleaseNote string `json:"release_note"`
	DownloadURL string `json:"download_url"`
	AssetName   string `json:"asset_name"`
	AssetSize   int64  `json:"asset_size"`
}

// ApplyCDNPrefix applies CDN prefix to download URL
func ApplyCDNPrefix(downloadURL, cdnPrefix string) string {
	if cdnPrefix == "" || downloadURL == "" {
		return downloadURL
	}
	return strings.TrimRight(cdnPrefix, "/") + "/" + downloadURL
}

func SetVersion(v string) {
	if v != "" {
		Version = v
	}
}

func SetBuildInfo(gitCommit, buildTime string) {
	if gitCommit != "" {
		GitCommit = gitCommit
	}
	if buildTime != "" {
		BuildTime = buildTime
	}
}

func GetInfo() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

func newGitHubRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", GitHubAPIVersion)
	req.Header.Set("User-Agent", GitHubUserAgent)
	return req, nil
}

func GetLatestRelease() (*ReleaseInfo, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	latestURL := fmt.Sprintf("%s/repos/%s/%s/releases/latest", APIBaseURL, RepoOwner, RepoName)
	req, err := newGitHubRequest(latestURL)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := client.Do(req)
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

	resp.Body.Close()

	listURL := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=1", APIBaseURL, RepoOwner, RepoName)
	req, err = newGitHubRequest(listURL)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err = client.Do(req)
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

func CheckUpdate(component string) (*UpdateInfo, error) {
	release, err := GetLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("get latest release: %w", err)
	}

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

func CheckUpdateForPlatform(component, osName, arch string) (*UpdateInfo, error) {
	release, err := GetLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("get latest release: %w", err)
	}

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

func findAssetForPlatform(assets []ReleaseAsset, component, osName, arch string) *ReleaseAsset {
	prefix := fmt.Sprintf("gotunnel-%s-", component)
	suffix := fmt.Sprintf("-%s-%s", osName, arch)

	for i := range assets {
		name := assets[i].Name
		if strings.HasPrefix(name, prefix) && strings.Contains(name, suffix) {
			return &assets[i]
		}
	}
	return nil
}

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
