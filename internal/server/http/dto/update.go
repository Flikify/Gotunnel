package dto

// CheckUpdateResponse 检查更新响应
// @Description 更新检查结果
type CheckUpdateResponse struct {
	Available   bool   `json:"available"`
	Current     string `json:"current"`
	Latest      string `json:"latest,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	ReleaseNote string `json:"release_note,omitempty"`
	AssetName   string `json:"asset_name,omitempty"`
	AssetSize   int64  `json:"asset_size,omitempty"`
}

// CheckClientUpdateQuery 检查客户端更新查询参数
// @Description 检查客户端更新的查询参数
type CheckClientUpdateQuery struct {
	OS   string `form:"os" binding:"omitempty,oneof=linux darwin windows"`
	Arch string `form:"arch" binding:"omitempty,oneof=amd64 arm64 386 arm"`
}

// ApplyServerUpdateRequest 应用服务端更新请求
// @Description 应用服务端更新
type ApplyServerUpdateRequest struct {
	DownloadURL   string `json:"download_url" binding:"required,url"`
	TargetVersion string `json:"target_version,omitempty"`
	Restart       bool   `json:"restart"`
}

// ApplyClientUpdateRequest 应用客户端更新请求
// @Description 推送更新到客户端
type ApplyClientUpdateRequest struct {
	ClientID    string `json:"client_id" binding:"required"`
	DownloadURL string `json:"download_url" binding:"required,url"`
}

// ServerUpdateStatusResponse 服务端自更新状态
// @Description 服务端自更新任务状态
type ServerUpdateStatusResponse struct {
	State          string `json:"state"`
	Message        string `json:"message,omitempty"`
	CurrentVersion string `json:"current_version,omitempty"`
	TargetVersion  string `json:"target_version,omitempty"`
	StartedAt      int64  `json:"started_at,omitempty"`
	FinishedAt     int64  `json:"finished_at,omitempty"`
	UpdatedAt      int64  `json:"updated_at,omitempty"`
}

// VersionInfo 版本信息
// @Description 当前版本信息
type VersionInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit,omitempty"`
	BuildTime string `json:"build_time,omitempty"`
	GoVersion string `json:"go_version,omitempty"`
	OS        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
}

// StatusResponse 服务器状态响应
// @Description 服务器状态信息
type StatusResponse struct {
	Server      ServerStatus `json:"server"`
	ClientCount int          `json:"client_count"`
}

// ServerStatus 服务器状态
type ServerStatus struct {
	BindAddr string `json:"bind_addr"`
	BindPort int    `json:"bind_port"`
}

// LoginRequest 登录请求
// @Description 用户登录
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
// @Description 登录成功返回
type LoginResponse struct {
	Token string `json:"token"`
}

// TokenCheckResponse Token 检查响应
// @Description Token 验证结果
type TokenCheckResponse struct {
	Valid    bool   `json:"valid"`
	Username string `json:"username"`
}
