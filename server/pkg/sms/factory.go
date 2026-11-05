package sms

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrProviderNotFound 表示配置的短信厂商没有对应驱动
var ErrProviderNotFound = errors.New("短信厂商不支持")

// ErrAccountNotFound 表示配置里没有对应短信账号
var ErrAccountNotFound = errors.New("短信账号不存在")

// provider 是一个短信厂商驱动，返回结果中的账号信息由上层填充
type provider interface {
	// send 发送一条模板短信，params 为模板变量
	send(ctx context.Context, phone, templateCode string, params map[string]string) (SendResult, error)
}

// builder 按账号配置构建驱动，timeout 为单次调用超时
type builder func(account Account, timeout time.Duration) (provider, error)

var (
	// registryMu 保护 registry，驱动只在包初始化阶段注册
	registryMu sync.RWMutex
	// registry 是厂商名到驱动构建函数的注册表
	registry = map[string]builder{}
)

// register 注册一个厂商驱动，重复注册以最后一次为准
func register(name string, build builder) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = build
}

// driver 按厂商名取驱动构建函数
func driver(name string) (builder, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	build, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}
	return build, nil
}

// sender 是一个账号的配置与驱动
type sender struct {
	account  Account
	provider provider
}

// senderFactory 持有全部账号的驱动实例，Init 时一次写入，请求期只读
type senderFactory struct {
	mu             sync.RWMutex
	senders        map[string]sender
	defaultAccount string
}

// newSenderFactory 创建空的驱动工厂
func newSenderFactory() *senderFactory {
	return &senderFactory{senders: map[string]sender{}}
}

// build 按配置构建全部账号驱动，构建失败直接返回错误，保证启动即失败
func (f *senderFactory) build(smsCfg Config, timeout time.Duration) error {
	senders := make(map[string]sender, len(smsCfg.Accounts))
	for name, account := range smsCfg.Accounts {
		name = strings.TrimSpace(name)
		if name == "" {
			return errors.New("短信账号名不能为空")
		}
		account.Provider = strings.TrimSpace(account.Provider)
		build, err := driver(account.Provider)
		if err != nil {
			return fmt.Errorf("短信账号[%s]: %w", name, err)
		}
		instance, err := build(account, timeout)
		if err != nil {
			return fmt.Errorf("短信账号[%s]: %w", name, err)
		}
		senders[name] = sender{account: account, provider: instance}
	}
	if len(senders) == 0 {
		return errors.New("短信已启用但未配置任何账号")
	}

	defaultAccount := strings.TrimSpace(smsCfg.Default)
	if defaultAccount == "" {
		if len(senders) != 1 {
			return errors.New("短信已启用但未配置默认账号")
		}
		for name := range senders {
			defaultAccount = name
		}
	}
	if _, ok := senders[defaultAccount]; !ok {
		return fmt.Errorf("短信默认账号[%s]不存在", defaultAccount)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.senders = senders
	f.defaultAccount = defaultAccount
	return nil
}

// get 按账号名取驱动，name 为空时用默认账号
func (f *senderFactory) get(name string) (string, sender, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if strings.TrimSpace(name) == "" {
		name = f.defaultAccount
	}
	item, ok := f.senders[name]
	if !ok {
		return "", sender{}, fmt.Errorf("%w: %s", ErrAccountNotFound, name)
	}
	return name, item, nil
}
