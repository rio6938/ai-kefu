package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"admin/server/pkg/config"
)

// ErrInvalidToken 是 token 解析失败时返回的错误
var ErrInvalidToken = errors.New("token 无效或已过期")

// secret 是签发和校验 token 用的密钥，Init 之后可用
var secret []byte

// userSecret 是签发和校验前台用户 token 用的密钥，InitUser 之后可用
var userSecret []byte

// expire 是 token 有效期
var expire time.Duration

// Claims 是签发到 token 里的业务信息
type Claims struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Init 按配置初始化密钥与有效期
func Init(cfg config.JWT) {
	secret = []byte(cfg.Secret)
	expire = time.Duration(cfg.Expire) * time.Second
}

// InitUser 按配置初始化前台用户 token 的密钥
func InitUser(cfg config.JWT) {
	userSecret = []byte(cfg.UserSecret)
}

// Generate 签发 token
func Generate(userID int64, username string) (string, error) {
	return generate(secret, userID, username)
}

// GenerateUser 签发前台用户 token，使用独立密钥，与后台 token 互不通用
func GenerateUser(userID int64, username string) (string, error) {
	return generate(userSecret, userID, username)
}

// Parse 校验并解析 token
func Parse(token string) (*Claims, error) {
	return parse(secret, token)
}

// ParseUser 校验并解析前台用户 token
func ParseUser(token string) (*Claims, error) {
	return parse(userSecret, token)
}

// generate 用指定密钥签发 token
func generate(key []byte, userID int64, username string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expire)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
}

// parse 用指定密钥校验并解析 token
func parse(key []byte, token string) (*Claims, error) {
	claims := new(Claims)
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return key, nil
	})
	if err != nil || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
