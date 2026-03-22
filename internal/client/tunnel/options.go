package tunnel

import "time"

// PlatformFeatures controls which platform-specific capabilities the client may use.
type PlatformFeatures struct {
	AllowSelfUpdate   bool
	AllowScreenshot   bool
	AllowShellExecute bool
	AllowSystemStats  bool
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
	return PlatformFeatures{
		AllowSelfUpdate:   true,
		AllowScreenshot:   true,
		AllowShellExecute: true,
		AllowSystemStats:  true,
	}
}

// MobilePlatformFeatures disables capabilities that are unsuitable for a mobile sandbox.
func MobilePlatformFeatures() PlatformFeatures {
	return PlatformFeatures{
		AllowSelfUpdate:   false,
		AllowScreenshot:   false,
		AllowShellExecute: false,
		AllowSystemStats:  true,
	}
}
