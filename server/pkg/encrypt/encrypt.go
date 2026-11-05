// Package encrypt 提供配置中心里密钥类配置的对称加解密，
// 主密钥来自配置文件的 [security] config_key，密文只在服务端解密
package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"admin/server/pkg/config"
	"admin/server/pkg/log"
)

// prefix 是密文的前缀，带前缀的值才需要解密，其余按明文处理，
// 兼容手工写入数据库或历史遗留的明文数据
const prefix = "enc:v1:"

// keySize 是 AES-256 的密钥长度
const keySize = 32

// ErrKeyNotConfigured 是未配置加密主密钥
var ErrKeyNotConfigured = errors.New("未配置 security.config_key，无法保存密钥类配置")

// key 是加密主密钥，Init 之后可用，为空表示未配置
var key []byte

// Init 解析加密主密钥，32 字节的 base64 直接使用，其它值按 sha256 派生为 32 字节，
// 未配置时不返回错误，只是无法保存密钥类配置
func Init(cfg config.Security) error {
	value := strings.TrimSpace(cfg.ConfigKey)
	if value == "" {
		key = nil
		log.Logger.Warn("未配置 security.config_key，密钥类配置将无法保存")
		return nil
	}

	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && len(decoded) == keySize {
		key = decoded
		return nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil && len(decoded) == keySize {
		key = decoded
		return nil
	}

	// 任意口令按 sha256 派生，避免强制要求固定的密钥格式
	sum := sha256.Sum256([]byte(value))
	key = sum[:]
	return nil
}

// IsEncrypted 判断配置值是否已经加密
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, prefix)
}

// Encrypt 加密配置值，返回值带 enc:v1: 前缀，可直接写入数据库
func Encrypt(plain string) (string, error) {
	aead, err := newGCM()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("生成加密随机数失败: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, []byte(plain), nil)
	return prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt 解密配置值，不带前缀的值按明文原样返回
func Decrypt(value string) (string, error) {
	if !IsEncrypted(value) {
		return value, nil
	}
	aead, err := newGCM()
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", fmt.Errorf("配置值不是合法的密文: %w", err)
	}
	if len(data) < aead.NonceSize() {
		return "", errors.New("配置值的密文长度不正确")
	}
	plain, err := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("配置值解密失败: %w", err)
	}
	return string(plain), nil
}

// Mask 生成密钥类配置的掩码，只保留末尾 4 个字符
func Mask(plain string) string {
	runes := []rune(plain)
	if len(runes) == 0 {
		return ""
	}
	if len(runes) <= 4 {
		return "****"
	}
	return "****" + string(runes[len(runes)-4:])
}

// Derive 用主密钥派生其它用途的密钥，label 不同则结果不同，
// 用于短信摘要密钥这类不需要由运维维护的内置密钥
func Derive(label string) ([]byte, error) {
	if len(key) == 0 {
		return nil, ErrKeyNotConfigured
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(label))
	return mac.Sum(nil), nil
}

// newGCM 按主密钥创建 AES-GCM，未配置主密钥时返回 ErrKeyNotConfigured
func newGCM() (cipher.AEAD, error) {
	if len(key) == 0 {
		return nil, ErrKeyNotConfigured
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建加密算法失败: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建加密模式失败: %w", err)
	}
	return aead, nil
}
