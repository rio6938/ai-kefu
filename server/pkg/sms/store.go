package sms

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// 短信相关数据在 redis 中的 key 前缀
const (
	codePrefix     = "sms:code:"     // 验证码摘要
	cooldownPrefix = "sms:cooldown:" // 重发冷却
	hourlyPrefix   = "sms:hourly:"   // 手机号每小时计数
	dailyPrefix    = "sms:daily:"    // 手机号自然日计数
	attemptPrefix  = "sms:attempt:"  // 验证码错误次数
	claimPrefix    = "sms:claim:"    // 验证码一次性占用
)

// redisTimeout 是单次 redis 操作的最长耗时
const redisTimeout = 5 * time.Second

// incrScript 原子地累加计数，只在首次设置过期时间，避免并发下出现没有过期时间的 key
var incrScript = goredis.NewScript(`
local value = redis.call('INCR', KEYS[1])
if value == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return value
`)

// decrScript 原子地回滚计数，减到 0 或负数时直接删除，避免残留没有过期时间的 key
var decrScript = goredis.NewScript(`
local value = redis.call('DECR', KEYS[1])
if value <= 0 then
  redis.call('DEL', KEYS[1])
end
return value
`)

// codeKey 返回验证码在 redis 中的 key，digest 为手机号与场景的摘要
func codeKey(digest string) string { return codePrefix + digest }

// cooldownKey 返回重发冷却的 key
func cooldownKey(digest string) string { return cooldownPrefix + digest }

// attemptKey 返回验证码错误次数的 key
func attemptKey(digest string) string { return attemptPrefix + digest }

// claimKey 返回验证码一次性占用的 key
func claimKey(digest string) string { return claimPrefix + digest }

// hourlyKey 返回手机号小时计数的 key
func hourlyKey(phone string) string { return hourlyPrefix + phoneDigest(phone) }

// dailyKey 返回手机号自然日计数的 key
func dailyKey(phone string) string { return dailyPrefix + phoneDigest(phone) }

// digestOf 计算手机号与场景的摘要，作为 redis key 后缀，避免落明文手机号
func digestOf(phone, scene string) string {
	return hmacHex(phone + "|" + scene)
}

// phoneDigest 计算手机号摘要，用于跨场景的手机号维度限流
func phoneDigest(phone string) string { return hmacHex(phone) }

// hmacHex 用配置密钥对 value 做 HMAC-SHA256 并转十六进制
func hmacHex(value string) string {
	mac := hmac.New(sha256.New, currentActive().secret)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

// hmacEqual 按固定时间比较两个摘要，避免时序攻击
func hmacEqual(left, right string) bool {
	return hmac.Equal([]byte(left), []byte(right))
}

// maskPhone 对手机号脱敏，日志只保留前三位和后四位
func maskPhone(phone string) string {
	if len(phone) <= 7 {
		return "***"
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

// setNX 在 key 不存在时写入并设置过期时间，用于重发冷却占位
func setNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	c, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()
	return client.SetNX(c, key, value, ttl).Result()
}

// incrWithExpire 原子累加计数，只在首次写入时设置过期时间
func incrWithExpire(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	c, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()
	return incrScript.Run(c, client, []string{key}, ttl.Milliseconds()).Int64()
}

// saveCode 保存验证码摘要，不保存明文
func saveCode(ctx context.Context, digest, code string) error {
	c, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()
	return client.Set(c, codeKey(digest), hmacHex(code), time.Duration(currentActive().cfg.Expire)*time.Second).Err()
}

// readCode 读取验证码摘要，不存在时返回 redis.Nil
func readCode(ctx context.Context, digest string) (string, error) {
	c, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()
	return client.Get(c, codeKey(digest)).Result()
}

// del 删除若干 key
func del(ctx context.Context, keys ...string) error {
	c, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()
	return client.Del(c, keys...).Err()
}

// decrOrDefault 原子回滚计数，减到 0 时删除 key
func decrOrDefault(ctx context.Context, key string) error {
	c, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()
	return decrScript.Run(c, client, []string{key}).Err()
}

// dailyTTL 返回到次日 0 点剩余的时间，作为自然日计数的过期时间
func dailyTTL() time.Duration {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return next.Sub(now)
}

// trimPhone 去掉手机号里的空格，兼容用户输入
func trimPhone(phone string) string {
	return strings.ReplaceAll(strings.TrimSpace(phone), " ", "")
}
