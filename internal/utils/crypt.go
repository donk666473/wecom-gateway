// Package utils 提供加解密工具函数。
// 参照钉钉对接项目中的 utils/crypt 模块和 DATRIX 免密登录算法。
package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

// AESEncrypt AES-CBC 加密（PKCS7 填充）
// key 和 iv 为原始字节，plain 为明文。
func AESEncrypt(key, iv, plain []byte) (string, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return "", fmt.Errorf("invalid AES key length: %d", len(key))
	}
	if len(iv) != aes.BlockSize {
		return "", fmt.Errorf("invalid AES IV length: %d", len(iv))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	plain = pkcs7Padding(plain, aes.BlockSize)
	ciphertext := make([]byte, len(plain))

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plain)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// AESDecrypt AES-CBC 解密（PKCS7 填充）
func AESDecrypt(key, iv []byte, ciphertext string) ([]byte, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, fmt.Errorf("invalid AES key length: %d", len(key))
	}
	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("invalid AES IV length: %d", len(iv))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}

	if len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plain := make([]byte, len(data))
	mode.CryptBlocks(plain, data)

	return pkcs7UnPadding(plain)
}

// pkcs7Padding PKCS7 填充
func pkcs7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := make([]byte, padding)
	for i := range padText {
		padText[i] = byte(padding)
	}
	return append(data, padText...)
}

// pkcs7UnPadding PKCS7 去填充
func pkcs7UnPadding(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("empty data")
	}
	padLen := int(data[length-1])
	if padLen == 0 || padLen > aes.BlockSize || padLen > length {
		return nil, fmt.Errorf("invalid padding length: %d", padLen)
	}
	for i := length - padLen; i < length; i++ {
		if data[i] != byte(padLen) {
			return nil, fmt.Errorf("invalid padding bytes")
		}
	}
	return data[:length-padLen], nil
}

// Md5Sum 计算 MD5 哈希
func Md5Sum(data string) string {
	h := md5.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// Sha1Sum 计算 SHA1 哈希
func Sha1Sum(data []byte) string {
	h := sha1.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// HmacSHA256 计算 HMAC-SHA256
func HmacSHA256(data, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// GenerateNonce 生成随机 Nonce（16 字节 hex 编码）
func GenerateNonce() string {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// DecryptPassword 使用 AES-CBC 解密加密的密码。
// 用于解密配置文件中加密存储的数据库/Redis 密码。
// key 和 iv 需根据实际环境配置。
func DecryptPassword(encrypted, keyHex, ivHex string) (string, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", fmt.Errorf("invalid key hex: %w", err)
	}
	iv, err := hex.DecodeString(ivHex)
	if err != nil {
		return "", fmt.Errorf("invalid iv hex: %w", err)
	}
	decrypted, err := AESDecrypt(key, iv, encrypted)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}

// DecryptCompat 兼容 dingtalkrobot 项目的 AES-CBC 密码解密。
// 使用与 dingtalkrobot 相同的默认密钥，自动处理 base64 编码的加密密码。
// 如果解密失败，返回原始字符串（兼容明文密码）。
func DecryptCompat(encrypted string) string {
	if encrypted == "" {
		return encrypted
	}

	// 检查是否为 base64 加密格式（结尾有 = 或 ==）
	isBase64Encoded := false
	for _, c := range encrypted {
		if c == '=' {
			isBase64Encoded = true
			break
		}
	}
	if !isBase64Encoded {
		return encrypted // 明文密码，直接返回
	}

	// 与 dingtalkrobot 相同的 AES key 和 IV
	key := []byte("b558a55ed617dab7")
	iv := []byte("cdccB3uiWDu7mcxw")

	plainBytes, err := AESDecrypt(key, iv, encrypted)
	if err != nil {
		return encrypted // 解密失败，当作明文使用
	}
	// 去除尾部可能的 \x00 填充
	plain := string(plainBytes)
	plain = trimNullBytes(plain)
	return plain
}

// trimNullBytes 去除字符串尾部的 null 字节。
func trimNullBytes(s string) string {
	for len(s) > 0 && s[len(s)-1] == 0 {
		s = s[:len(s)-1]
	}
	return s
}
