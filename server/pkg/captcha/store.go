package captcha

import (
	"context"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// 验证码在 redis 中的 key 前缀
const keyPrefix = "captcha:"

// store 用 redis 保存验证码答案，实现 base64Captcha 的 Store 接口
type store struct {
	client *goredis.Client
	expire time.Duration
}

// Set 保存验证码答案
func (s *store) Set(id string, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.client.Set(ctx, keyPrefix+id, value, s.expire).Err()
}

// Get 读取验证码答案，clear 为 true 时删除（验证码只能用一次）
func (s *store) Get(id string, clear bool) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := keyPrefix + id
	if clear {
		value, err := s.client.GetDel(ctx, key).Result()
		if err != nil {
			return ""
		}
		return value
	}

	value, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return ""
	}
	return value
}

// Verify 校验验证码答案
func (s *store) Verify(id, answer string, clear bool) bool {
	return strings.EqualFold(s.Get(id, clear), answer)
}
