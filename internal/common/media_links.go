// Package common 提供媒体链接处理功能。
// 参照设计文档 5.2 节媒体链接转换流和 4.7.2 节。
//
// DATRIX 返回的 Markdown 中包含相对路径的图片和链接，
// 需要转换为完整的外部可访问 URL。
package common

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/wecom-gateway/internal/utils"
)

// ProcessMediaLinks 处理 Markdown 中的媒体链接。
// 将 DATRIX 返回的相对路径图片和链接转换为完整 URL。
func ProcessMediaLinks(content, token string) string {
	server := DatrixAssistantURL

	// 1. 替换图片 ![alt](src)
	imgRe := regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)
	content = imgRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := imgRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		altText := parts[1]
		imgSrc := parts[2]
		processedURL := buildCommonImageURL(imgSrc, token, server)
		if processedURL != imgSrc {
			return fmt.Sprintf("![%s](%s)", altText, processedURL)
		}
		return match
	})

	// 2. 替换链接 [text](href)（排除已处理的图片）
	linkRe := regexp.MustCompile(`(?<!!)\[(.*?)\]\((.*?)\)`)
	content = linkRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := linkRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		linkText := parts[1]
		linkURL := parts[2]
		processedURL := buildCommonLinkURL(linkURL, server)
		if processedURL != linkURL {
			return fmt.Sprintf("[%s](%s)", linkText, processedURL)
		}
		return match
	})

	return content
}

// buildCommonImageURL 构建图片预览完整 URL。
func buildCommonImageURL(src, token, server string) string {
	if strings.Contains(src, "://") {
		return src
	}

	parts := strings.Split(src, "/")
	if len(parts) != 2 {
		return server + src
	}

	resourceID := parts[0]
	imageName := parts[1]
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	sign := calcCommonSign(resourceID, imageName, nonce, token)

	return fmt.Sprintf("%s/api/mx/api/v1/file/_preview?resource_id=%s&image_name=%s&token=%s&nonce=%s&sign=%s",
		server, resourceID, imageName, token, nonce, sign)
}

// buildCommonLinkURL 构建链接完整 URL。
func buildCommonLinkURL(src, server string) string {
	if strings.Contains(src, "://") {
		return src
	}
	return server + src
}

// calcCommonSign 计算图片签名（MD5）
func calcCommonSign(resourceID, imageName, nonce, token string) string {
	data := fmt.Sprintf("%s%s%s%s", resourceID, imageName, nonce, token)
	return utils.Md5Sum(data)
}
