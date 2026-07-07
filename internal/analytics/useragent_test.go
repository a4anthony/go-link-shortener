package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseUserAgent(t *testing.T) {
	tests := []struct {
		name    string
		ua      string
		browser string
		os      string
		device  string
	}{
		{
			name:    "chrome on windows",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36",
			browser: "Chrome", os: "Windows", device: DeviceDesktop,
		},
		{
			name:    "safari on iphone",
			ua:      "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			browser: "Safari", os: "iOS", device: DeviceMobile,
		},
		{
			name:    "chrome on android",
			ua:      "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Mobile Safari/537.36",
			browser: "Chrome", os: "Android", device: DeviceMobile,
		},
		{
			name:    "safari on ipad",
			ua:      "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1 Version/17.0 Safari/604.1",
			browser: "Safari", os: "iOS", device: DeviceTablet,
		},
		{
			name:    "firefox on linux",
			ua:      "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
			browser: "Firefox", os: "Linux", device: DeviceDesktop,
		},
		{
			name:    "edge on windows",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36 Edg/120.0",
			browser: "Edge", os: "Windows", device: DeviceDesktop,
		},
		{
			name:    "safari on macos",
			ua:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1 (KHTML, like Gecko) Version/17.0 Safari/605.1",
			browser: "Safari", os: "macOS", device: DeviceDesktop,
		},
		{
			name:    "googlebot",
			ua:      "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			browser: "bot", os: Unknown, device: DeviceBot,
		},
		{
			name:    "curl",
			ua:      "curl/8.4.0",
			browser: "bot", os: Unknown, device: DeviceBot,
		},
		{
			name:    "empty",
			ua:      "",
			browser: Unknown, os: Unknown, device: Unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseUserAgent(tt.ua)
			assert.Equal(t, tt.browser, got.Browser, "browser")
			assert.Equal(t, tt.os, got.OS, "os")
			assert.Equal(t, tt.device, got.Device, "device")
		})
	}
}
