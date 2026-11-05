package common

import "strings"

// ParseDevice 从 User-Agent 粗略解析设备型号
func ParseDevice(userAgent string) string {
	if userAgent == "" {
		return "Unknown"
	}
	if strings.Contains(userAgent, "Mobile") || strings.Contains(userAgent, "Android") || strings.Contains(userAgent, "iPhone") {
		return "Mobile"
	}
	return "PC"
}

// ParseOS 从 User-Agent 粗略解析操作系统
func ParseOS(userAgent string) string {
	switch {
	case strings.Contains(userAgent, "Windows"):
		return "Windows"
	case strings.Contains(userAgent, "Mac OS"):
		return "Mac OS"
	case strings.Contains(userAgent, "Android"):
		return "Android"
	case strings.Contains(userAgent, "iPhone") || strings.Contains(userAgent, "iPad"):
		return "iOS"
	case strings.Contains(userAgent, "Linux"):
		return "Linux"
	default:
		return "Unknown"
	}
}

// ParseBrowser 从 User-Agent 粗略解析浏览器
func ParseBrowser(userAgent string) string {
	switch {
	case strings.Contains(userAgent, "Edg/"):
		return "Edge"
	case strings.Contains(userAgent, "Chrome/"):
		return "Chrome"
	case strings.Contains(userAgent, "Firefox/"):
		return "Firefox"
	case strings.Contains(userAgent, "Safari/"):
		return "Safari"
	default:
		return "Unknown"
	}
}
