package sms

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"admin/server/pkg/encrypt"
	"admin/server/pkg/log"
	"admin/server/pkg/setting"
)

// 导出错误，调用方通过 errors.Is 判断后映射成业务提示
var (
	ErrDisabled         = errors.New("短信功能未启用")
	ErrPhoneRequired    = errors.New("手机号不能为空")
	ErrSceneRequired    = errors.New("短信场景不能为空")
	ErrTemplateNotFound = errors.New("短信模板未配置")
	ErrTooFrequent      = errors.New("验证码发送过于频繁，请稍后再试")
	ErrHourlyLimit      = errors.New("验证码发送次数已达上限，请稍后再试")
	ErrDailyLimit       = errors.New("验证码发送次数已达上限，请明天再试")
	ErrSendFailed       = errors.New("短信发送失败")
	ErrCodeExpired      = errors.New("验证码已过期，请重新获取")
	ErrCodeIncorrect    = errors.New("验证码错误")
	ErrAttemptLimit     = errors.New("验证码错误次数过多，请重新获取")
)

// SendResult 是一条短信的发送结果
type SendResult struct {
	// Account 是发送使用的短信账号名
	Account string `json:"account"`
	// Provider 是短信厂商，如 aliyun / tencent
	Provider string `json:"provider"`
	// BizID 是服务商返回的流水号，便于排查问题
	BizID string `json:"bizId"`
}

// Result 是发送验证码的返回结构
type Result struct {
	// Code 是验证码文本，只有 debug 模式才有值
	Code string `json:"code"`
}

// active 是一次配置生效后的运行快照，内容只读，配置变更时整体替换
type active struct {
	// cfg 是短信配置
	cfg Config
	// secret 是手机号与验证码的摘要密钥
	secret []byte
}

var (
	// factory 是全局驱动工厂，Init 之后可用
	factory *senderFactory

	// client 是验证码与限流使用的 redis 客户端
	client *goredis.Client

	// current 是当前生效的运行快照，请求期无锁读取
	current atomic.Pointer[active]

	// debug 为 true 时不真正发送，把验证码写日志并返回给调用方
	debug bool
)

// currentActive 返回当前生效的运行快照，未初始化时返回零值
func currentActive() *active {
	value := current.Load()
	if value == nil {
		return &active{}
	}
	return value
}

// Init 从配置中心读取短信配置并初始化驱动工厂，同时注册配置变更后的重建回调
func Init(isDebug bool, redisClient *goredis.Client) error {
	debug = isDebug
	client = redisClient
	setting.OnReload(func() {
		if err := apply(); err != nil {
			log.Logger.Error("短信配置变更后重建失败", zap.Error(err))
		}
	})
	return apply()
}

// apply 读取配置中心的短信配置并重建驱动工厂，校验失败时保持原有配置不变
func apply() error {
	cfg := loadConfig()

	// 未启用时允许空配置启动，只有调用发送接口才返回 ErrDisabled
	if !cfg.Enabled {
		current.Store(&active{cfg: cfg})
		factory = newSenderFactory()
		return nil
	}
	if client == nil {
		return errors.New("短信已启用但 redis 未初始化")
	}
	hmacSecret, err := encrypt.Derive("sms-hmac")
	if err != nil {
		return fmt.Errorf("初始化短信摘要密钥失败: %w", err)
	}
	if cfg.CodeLength < 4 || cfg.CodeLength > 8 {
		return errors.New("短信验证码位数需在 4~8 之间")
	}
	if cfg.Expire <= 0 || cfg.Interval <= 0 || cfg.HourlyLimit <= 0 ||
		cfg.DailyLimit <= 0 || cfg.MaxAttempts <= 0 || cfg.Timeout <= 0 {
		return errors.New("短信有效期、发送间隔、次数限制与超时必须为正数")
	}

	instance := newSenderFactory()
	if err := instance.build(cfg, time.Duration(cfg.Timeout)*time.Second); err != nil {
		return err
	}

	// 全部校验通过后再替换快照，避免失败时新配置与旧驱动不一致
	current.Store(&active{cfg: cfg, secret: hmacSecret})
	factory = instance
	return nil
}

// Send 用指定账号发送模板短信，account 为空时用默认账号
func Send(ctx context.Context, account, phone, templateCode string, params map[string]string) (SendResult, error) {
	if !currentActive().cfg.Enabled {
		return SendResult{}, ErrDisabled
	}
	phone = trimPhone(phone)
	if phone == "" {
		return SendResult{}, ErrPhoneRequired
	}
	templateCode = strings.TrimSpace(templateCode)
	if templateCode == "" {
		return SendResult{}, fmt.Errorf("%w: 模板 ID 不能为空", ErrTemplateNotFound)
	}
	name, item, err := factory.get(account)
	if err != nil {
		return SendResult{}, err
	}
	result, err := item.provider.send(ctx, phone, templateCode, params)
	if err != nil {
		log.Logger.Warn("短信发送失败",
			zap.String("account", name),
			zap.String("provider", item.account.Provider),
			zap.String("phone", maskPhone(phone)),
			zap.Error(err),
		)
		return SendResult{}, fmt.Errorf("%w: %v", ErrSendFailed, err)
	}
	result.Account = name
	result.Provider = item.account.Provider
	return result, nil
}

// SendCode 发送指定场景的验证码，scene 对应账号配置里的模板键名；
// debug 模式不调用厂商，把验证码写日志并随结果返回
func SendCode(ctx context.Context, phone, scene string) (*Result, error) {
	snapshot := currentActive()
	if !snapshot.cfg.Enabled {
		return nil, ErrDisabled
	}
	phone = trimPhone(phone)
	if phone == "" {
		return nil, ErrPhoneRequired
	}
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return nil, ErrSceneRequired
	}

	name, item, err := factory.get("")
	if err != nil {
		return nil, err
	}
	templateCode := strings.TrimSpace(item.account.Templates[scene])
	if templateCode == "" {
		return nil, fmt.Errorf("%w: %s", ErrTemplateNotFound, scene)
	}

	digest := digestOf(phone, scene)
	if err := allowSend(ctx, phone, digest); err != nil {
		return nil, err
	}

	code, err := randomCode(snapshot.cfg.CodeLength)
	if err != nil {
		releaseSend(ctx, phone, digest)
		return nil, err
	}
	if err := saveCode(ctx, digest, code); err != nil {
		releaseSend(ctx, phone, digest)
		return nil, fmt.Errorf("保存短信验证码失败: %w", err)
	}

	if debug {
		log.Logger.Info("短信验证码（调试模式，未实际发送）",
			zap.String("account", name),
			zap.String("scene", scene),
			zap.String("phone", maskPhone(phone)),
			zap.String("code", code),
		)
		return &Result{Code: code}, nil
	}

	result, err := item.provider.send(ctx, phone, templateCode, map[string]string{"code": code})
	if err != nil {
		releaseSend(ctx, phone, digest)
		log.Logger.Warn("短信验证码发送失败",
			zap.String("account", name),
			zap.String("provider", item.account.Provider),
			zap.String("scene", scene),
			zap.String("phone", maskPhone(phone)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("%w: %v", ErrSendFailed, err)
	}
	log.Logger.Info("短信验证码已发送",
		zap.String("account", name),
		zap.String("provider", item.account.Provider),
		zap.String("scene", scene),
		zap.String("phone", maskPhone(phone)),
		zap.String("bizId", result.BizID),
	)
	return &Result{}, nil
}

// VerifyCode 校验指定场景的验证码，成功即作废并返回 nil；
// 失败返回 ErrCodeExpired / ErrCodeIncorrect / ErrAttemptLimit 供调用方区分提示
func VerifyCode(ctx context.Context, phone, scene, code string) error {
	snapshot := currentActive()
	if !snapshot.cfg.Enabled {
		return ErrDisabled
	}
	phone = trimPhone(phone)
	if phone == "" {
		return ErrPhoneRequired
	}
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return ErrSceneRequired
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrCodeIncorrect
	}

	digest := digestOf(phone, scene)
	stored, err := readCode(ctx, digest)
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return ErrCodeExpired
		}
		return fmt.Errorf("读取短信验证码失败: %w", err)
	}

	if !hmacEqual(stored, hmacHex(code)) {
		attempts, err := incrWithExpire(ctx, attemptKey(digest), time.Duration(snapshot.cfg.Expire)*time.Second)
		if err != nil {
			return fmt.Errorf("记录短信验证码错误次数失败: %w", err)
		}
		if attempts >= int64(snapshot.cfg.MaxAttempts) {
			if err := del(ctx, codeKey(digest), attemptKey(digest)); err != nil {
				log.Logger.Warn("作废短信验证码失败",
					zap.String("scene", scene),
					zap.String("phone", maskPhone(phone)),
					zap.Error(err),
				)
			}
			return ErrAttemptLimit
		}
		return ErrCodeIncorrect
	}

	// claim 抢占保证并发请求下验证码只会校验成功一次
	claimed, err := incrWithExpire(ctx, claimKey(digest), time.Duration(snapshot.cfg.Expire)*time.Second)
	if err != nil {
		return fmt.Errorf("占用短信验证码失败: %w", err)
	}
	if claimed != 1 {
		return ErrCodeExpired
	}
	if err := del(ctx, codeKey(digest), attemptKey(digest)); err != nil {
		log.Logger.Warn("清理短信验证码失败",
			zap.String("scene", scene),
			zap.String("phone", maskPhone(phone)),
			zap.Error(err),
		)
	}
	return nil
}

// allowSend 做重发间隔、每小时与自然日限流，任一项超限返回对应错误
func allowSend(ctx context.Context, phone, digest string) error {
	snapshot := currentActive()

	ok, err := setNX(ctx, cooldownKey(digest), "1", time.Duration(snapshot.cfg.Interval)*time.Second)
	if err != nil {
		return fmt.Errorf("校验短信发送间隔失败: %w", err)
	}
	if !ok {
		return ErrTooFrequent
	}

	hourly, err := incrWithExpire(ctx, hourlyKey(phone), time.Hour)
	if err != nil {
		return fmt.Errorf("校验短信小时发送次数失败: %w", err)
	}
	if hourly > int64(snapshot.cfg.HourlyLimit) {
		return ErrHourlyLimit
	}

	daily, err := incrWithExpire(ctx, dailyKey(phone), dailyTTL())
	if err != nil {
		return fmt.Errorf("校验短信当日发送次数失败: %w", err)
	}
	if daily > int64(snapshot.cfg.DailyLimit) {
		return ErrDailyLimit
	}
	return nil
}

// releaseSend 在发送失败时回滚占用，避免用户被冷却与限流锁住
func releaseSend(ctx context.Context, phone, digest string) {
	if err := del(ctx, cooldownKey(digest), codeKey(digest)); err != nil {
		log.Logger.Warn("回滚短信发送占用失败",
			zap.String("phone", maskPhone(phone)),
			zap.Error(err),
		)
	}
	for _, key := range []string{hourlyKey(phone), dailyKey(phone)} {
		if err := decrOrDefault(ctx, key); err != nil {
			log.Logger.Warn("回滚短信发送计数失败",
				zap.String("phone", maskPhone(phone)),
				zap.Error(err),
			)
		}
	}
}

// randomCode 生成指定位数的纯数字验证码
func randomCode(length int) (string, error) {
	limit := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	value, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return "", fmt.Errorf("生成短信验证码失败: %w", err)
	}
	return fmt.Sprintf("%0*s", length, value.String()), nil
}
