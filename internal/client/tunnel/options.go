package tunnel

import (
	"runtime"
	"time"
)

// PlatformFeatures controls which platform-specific capabilities the client may use.
type PlatformFeatures struct {
	AllowSelfUpdate    bool
	AllowScreenshot    bool
	AllowSystemStats   bool
	AllowRemoteControl bool
}

// ClientOptions controls optional client runtime settings.
type ClientOptions struct {
	DataDir           string
	ClientID          string
	ClientName        string
	Features          *PlatformFeatures
	ReconnectDelay    time.Duration
	ReconnectMaxDelay time.Duration
}

// DefaultPlatformFeatures enables the desktop feature set.
func DefaultPlatformFeatures() PlatformFeatures {
	allowScreenshot := true
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		allowScreenshot = false
	}

	return PlatformFeatures{
		AllowSelfUpdate:    true,
		AllowScreenshot:    allowScreenshot,
		AllowSystemStats:   true,
		AllowRemoteControl: runtime.GOOS == "windows",
	}
}

// MobilePlatformFeatures disables capabilities that are unsuitable for a mobile sandbox.
func MobilePlatformFeatures() PlatformFeatures {
	return PlatformFeatures{
		AllowSelfUpdate:    false,
		AllowScreenshot:    false,
		AllowSystemStats:   true,
		AllowRemoteControl: false,
	}
}
