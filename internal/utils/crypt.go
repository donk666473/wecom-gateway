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

	return pkcs7UnPadding(plain), nil
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
func pkcs7UnPadding(data []byte) []byte {
	length := len(data)
	if length == 0 {
		return data
	}
	unPadding := int(data[length-1])
	if unPadding > length {
		return data
	}
	return data[:length-unPadding]
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
	_, _ = io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}

// DecryptPassword 使用 AES-CBC 解密加密的密码。
// 用于解密配置文件中加密存储的数据库/Redis 密码。
// key 默认: "b558a55ed617dab7", iv 默认: "cdccB3uiWDu7mcxw"
func DecryptPassword(encrypted string) (string, error) {
	key := []byte("b558a55ed617dab7")
	iv := []byte("cdccB3uiWDu7mcxw")
	decrypted, err := AESDecrypt(key, iv, encrypted)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}
