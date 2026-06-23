// Package utils 提供通用工具函数。
// 参照钉钉对接项目中的 utils/util 模块。
package utils

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GenerateUUID 生成 UUID v4 字符串（无连字符）
func GenerateUUID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

// GenerateUUIDWithDash 生成标准 UUID v4 字符串（带连字符）
func GenerateUUIDWithDash() string {
	return uuid.New().String()
}

// GetHostname 获取当前主机名
func GetHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// GetLocalIP 获取本机 IPv4 地址
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

// PathExists 检查路径是否存在
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

// NormalizeURL 规范化 URL，确保尾部无斜杠。
func NormalizeURL(url string) string {
	return strings.TrimRight(url, "/")
}

// Recent5Minutes 返回 5 分钟前的时间
func Recent5Minutes() time.Time {
	return time.Now().Add(-5 * time.Minute)
}

// BuildCacheKey 构建带前缀的 Redis 缓存 Key
func BuildCacheKey(prefix, key string) string {
	return fmt.Sprintf("%s:%s", prefix, key)
}
