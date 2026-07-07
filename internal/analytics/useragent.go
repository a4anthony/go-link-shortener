// Package analytics implements the asynchronous click-ingestion pipeline: a
// buffered channel drained by a worker pool that enriches events (user-agent
// parsing, geo lookup, IP anonymisation) and batch-writes them to Postgres.
// Enrichment happens on the workers, never on the redirect hot path.
package analytics

import "strings"

// UserAgent is the parsed breakdown of a raw User-Agent header.
type UserAgent struct {
	Browser string
	OS      string
	Device  string
}

// Device classifications.
const (
	DeviceDesktop = "desktop"
	DeviceMobile  = "mobile"
	DeviceTablet  = "tablet"
	DeviceBot     = "bot"
	Unknown       = "unknown"
)

// ParseUserAgent extracts a coarse browser/OS/device classification from a raw
// User-Agent string. It is intentionally lightweight (substring heuristics) —
// enough for analytics breakdowns without a heavyweight UA database.
func ParseUserAgent(raw string) UserAgent {
	if strings.TrimSpace(raw) == "" {
		return UserAgent{Browser: Unknown, OS: Unknown, Device: Unknown}
	}
	lower := strings.ToLower(raw)

	if isBot(lower) {
		return UserAgent{Browser: "bot", OS: detectOS(lower), Device: DeviceBot}
	}
	return UserAgent{
		Browser: detectBrowser(lower),
		OS:      detectOS(lower),
		Device:  detectDevice(lower),
	}
}

func isBot(ua string) bool {
	for _, marker := range []string{"bot", "crawler", "spider", "slurp", "curl", "wget", "python-requests", "headlesschrome"} {
		if strings.Contains(ua, marker) {
			return true
		}
	}
	return false
}

func detectBrowser(ua string) string {
	switch {
	case strings.Contains(ua, "edg"): // Edge (Chromium): "edg/"
		return "Edge"
	case strings.Contains(ua, "opr/") || strings.Contains(ua, "opera"):
		return "Opera"
	case strings.Contains(ua, "firefox"):
		return "Firefox"
	case strings.Contains(ua, "chrome") || strings.Contains(ua, "crios"):
		return "Chrome"
	case strings.Contains(ua, "safari"):
		return "Safari"
	default:
		return Unknown
	}
}

func detectOS(ua string) string {
	switch {
	case strings.Contains(ua, "windows"):
		return "Windows"
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") || strings.Contains(ua, "ios"):
		return "iOS"
	case strings.Contains(ua, "mac os x") || strings.Contains(ua, "macintosh"):
		return "macOS"
	case strings.Contains(ua, "linux"):
		return "Linux"
	default:
		return Unknown
	}
}

func detectDevice(ua string) string {
	switch {
	case strings.Contains(ua, "ipad") || strings.Contains(ua, "tablet"):
		return DeviceTablet
	case strings.Contains(ua, "mobi") || strings.Contains(ua, "iphone") || strings.Contains(ua, "android"):
		return DeviceMobile
	default:
		return DeviceDesktop
	}
}
