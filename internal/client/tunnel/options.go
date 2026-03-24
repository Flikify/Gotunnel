package tunnel

import "time"

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
	return PlatformFeatures{
		AllowSelfUpdate:    true,
		AllowScreenshot:    true,
		AllowSystemStats:   true,
		AllowRemoteControl: true,
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
