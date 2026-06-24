// Package adapter 提供企微消息加解密实现。
// 参照企微官方 WXBizMsgCrypt 加解密库：
// https://developer.work.weixin.qq.com/document/path/90930
//
// 加解密流程：
// 1. 签名验证：SHA1(sort(token, timestamp, nonce, encrypt))
// 2. 消息解密：AES-256-CBC + Base64
// 3. 消息加密：AES-256-CBC + Base64
package adapter

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
)

// ============================================================================
// 企微 XML 消息结构
// ============================================================================

// wecomEncryptedXML 企微回调加密 XML 结构。
// 企微 Webhook POST 请求体的 XML 格式。
type wecomEncryptedXML struct {
	XMLName    xml.Name `xml:"xml"`
	ToUserName string   `xml:"ToUserName"`
	AgentID    string   `xml:"AgentID"`
	Encrypt    string   `xml:"Encrypt"`
}

// wecomDecryptedXML 企微解密后消息结构。
// 解密后得到的 XML 消息体，包含实际的消息内容。
// ChatID 字段用于区分群聊（非空 == 群聊消息）。
type wecomDecryptedXML struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   string   `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgID        string   `xml:"MsgId"`
	AgentID      int      `xml:"AgentID"`
	Event        string   `xml:"Event"`
	EventKey     string   `xml:"EventKey"`
	ChatID       string   `xml:"ChatId"` // 群聊 ID，非空表示群聊消息
}

// ============================================================================
// 签名验证
// ============================================================================

// verifySignature 验证企微回调签名。
// 签名算法：SHA1(sort(token, timestamp, nonce, encrypt))
func (w *WeComAdapter) verifySignature(msgSignature, timestamp, nonce, encrypt string) bool {
	// 排序 token, timestamp, nonce, encrypt
	strs := sort.StringSlice{w.token, timestamp, nonce, encrypt}
	sort.Strings(strs)

	// SHA1 计算
	hash := sha1.New()
	hash.Write([]byte(strings.Join(strs, "")))
	signature := fmt.Sprintf("%x", hash.Sum(nil))

	return signature == msgSignature
}

// ============================================================================
// 消息解密
// ============================================================================

// decryptMsg 解密企微加密消息。
// 返回解密后的原始 XML 字节。
func (w *WeComAdapter) decryptMsg(encrypt, corpID string) ([]byte, error) {
	// 1. Base64 解码
	ciphertext, err := base64.StdEncoding.DecodeString(encrypt)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	// 2. AES-256-CBC 解密
	block, err := aes.NewCipher(w.aesKey)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher failed: %w", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// IV 为 aesKey 的前 16 字节
	iv := w.aesKey[:aes.BlockSize]

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// 3. 去除 PKCS7 填充
	plaintext, err = pkcs7Unpad(plaintext)
	if err != nil {
		return nil, fmt.Errorf("pkcs7 unpad failed: %w", err)
	}

	// 4. 去除 16 字节随机数 + 4 字节消息长度
	if len(plaintext) < 20 {
		return nil, fmt.Errorf("decrypted message too short")
	}

	// 读取消息长度（大端序）
	buf := bytes.NewBuffer(plaintext[16:20])
	var msgLen int32
	_ = binary.Read(buf, binary.BigEndian, &msgLen)

	// 提取消息体
	msgEnd := 20 + msgLen
	if int(msgEnd) > len(plaintext) {
		return nil, fmt.Errorf("invalid message length")
	}
	msg := plaintext[20:msgEnd]

	// 5. 验证 corpID
	receivedCorpID := string(plaintext[msgEnd:])
	if receivedCorpID != corpID {
		return nil, fmt.Errorf("corpID mismatch: expected %s, got %s", corpID, receivedCorpID)
	}

	return msg, nil
}

// ============================================================================
// 辅助函数
// ============================================================================

// decodeAESKey 将 Base64 编码的 EncodingAESKey 解码为 []byte。
// EncodingAESKey 长度为 43 字符，解码后为 32 字节（AES-256 密钥）。
func decodeAESKey(encodingAESKey string) ([]byte, error) {
	// 企微 EncodingAESKey 为 43 字符的 Base64，需要补 "=" 后再解码
	aesKey, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return nil, fmt.Errorf("decode aes key failed: %w", err)
	}
	if len(aesKey) != 32 {
		return nil, fmt.Errorf("invalid aes key length: expected 32, got %d", len(aesKey))
	}
	return aesKey, nil
}

// pkcs7Unpad 去除 PKCS7 填充。
func pkcs7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("empty data")
	}
	padLen := int(data[length-1])
	if padLen > length || padLen > aes.BlockSize {
		return nil, fmt.Errorf("invalid padding length: %d", padLen)
	}
	return data[:length-padLen], nil
}

// splitByBytes 按 UTF-8 字节数分段。
// 确保不在多字节字符（中文等）中间截断。
func splitByBytes(text string, limit int) []string {
	if limit <= 0 {
		return []string{text}
	}

	var parts []string
	runes := []rune(text)
	currentPart := ""

	for _, r := range runes {
		runeBytes := len(string(r))
		if len(currentPart)+runeBytes > limit {
			if currentPart != "" {
				parts = append(parts, currentPart)
				currentPart = ""
			}
		}
		currentPart += string(r)
	}

	if currentPart != "" {
		parts = append(parts, currentPart)
	}

	// 如果分段为空，返回全部内容
	if len(parts) == 0 {
		parts = append(parts, text)
	}

	return parts
}
