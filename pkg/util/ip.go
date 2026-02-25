// pkg/util/ip.go
package util

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// isTrustedProxy checks whether the remote address belongs to a trusted proxy network.
// Only when the direct connection comes from a trusted proxy should we inspect forwarding headers.
func isTrustedProxy(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	trustedRanges := []string{
		"127.0.0.0/8",
		"::1/128",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}
	for _, cidr := range trustedRanges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// extractIPFromHeader extracts and validates the first IP from a potentially comma-separated header value.
func extractIPFromHeader(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	candidate := strings.TrimSpace(parts[0])
	if net.ParseIP(candidate) != nil {
		return candidate
	}
	return ""
}

// GetRealClientIP 获取客户端真实IP地址
// 仅当直连来源是可信代理时，才检查转发头部，防止客户端直接伪造代理头部。
func GetRealClientIP(c *gin.Context) string {
	if !isTrustedProxy(c.Request.RemoteAddr) {
		return c.ClientIP()
	}

	headers := []string{
		"X-Forwarded-For",
		"X-Real-IP",
		"CF-Connecting-IP",
		"EO-Connecting-IP",
		"Ali-CDN-Real-IP",
		"True-Client-IP",
	}

	for _, header := range headers {
		if ip := extractIPFromHeader(c.GetHeader(header)); ip != "" {
			return ip
		}
	}

	return c.ClientIP()
}

// IsValidIP 验证IP地址是否有效
func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// IsPrivateIP 检查是否为私有IP地址
func IsPrivateIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// IPv4私有地址范围
	privateIPRanges := []string{
		"10.0.0.0/8",     // 10.0.0.0 - 10.255.255.255
		"172.16.0.0/12",  // 172.16.0.0 - 172.31.255.255
		"192.168.0.0/16", // 192.168.0.0 - 192.168.255.255
		"127.0.0.0/8",    // 本地回环
	}

	for _, cidr := range privateIPRanges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(parsedIP) {
			return true
		}
	}

	return false
}
