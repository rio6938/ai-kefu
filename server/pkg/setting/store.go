// Package setting 是数据库配置中心的内存视图，
// 启动时从数据库加载一次，配置保存成功后整体替换，读取不加锁也不访问数据库
package setting

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"

	"admin/server/pkg/log"
)

// Item 是配置项在内存缓存中的内容
type Item struct {
	// Value 是配置值，密钥类配置为解密后的明文，只在进程内使用
	Value string
	// IsSecret 标识密钥类配置，不能通过公开接口下发
	IsSecret bool
	// IsPublic 标识允许通过公开接口下发
	IsPublic bool
}

var (
	// items 是配置项缓存，整体替换，读取时无需加锁
	items atomic.Value

	// reloadMu 保护 reloaders
	reloadMu sync.RWMutex
	// reloaders 是配置变更后需要重建的依赖
	reloaders []func()
)

// OnReload 注册配置变更后的回调，用于重建依赖配置的客户端，
// 回调在缓存替换后同步执行，失败由回调自己记日志，不影响保存结果
func OnReload(fn func()) {
	if fn == nil {
		return
	}
	reloadMu.Lock()
	defer reloadMu.Unlock()
	reloaders = append(reloaders, fn)
}

// Replace 整体替换配置缓存并触发已注册的回调
func Replace(list map[string]Item) {
	if list == nil {
		list = map[string]Item{}
	}
	items.Store(list)

	reloadMu.RLock()
	callbacks := make([]func(), len(reloaders))
	copy(callbacks, reloaders)
	reloadMu.RUnlock()

	for _, fn := range callbacks {
		fn()
	}
}

// GetString 读取字符串配置，未配置时返回默认值
func GetString(key, def string) string {
	item, ok := load(key)
	if !ok {
		return def
	}
	return item.Value
}

// GetInt 读取整数配置，未配置或格式不正确时返回默认值
func GetInt(key string, def int) int {
	value := strings.TrimSpace(GetString(key, ""))
	if value == "" {
		return def
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Logger.Warn("配置项不是合法的整数",
			zap.String("key", key), zap.String("value", value))
		return def
	}
	return parsed
}

// GetFloat 读取浮点数配置，未配置或格式不正确时返回默认值
func GetFloat(key string, def float64) float64 {
	value := strings.TrimSpace(GetString(key, ""))
	if value == "" {
		return def
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		log.Logger.Warn("配置项不是合法的浮点数",
			zap.String("key", key), zap.String("value", value))
		return def
	}
	return parsed
}

// GetBool 读取布尔配置，未配置或格式不正确时返回默认值
func GetBool(key string, def bool) bool {
	value := strings.TrimSpace(GetString(key, ""))
	if value == "" {
		return def
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Logger.Warn("配置项不是合法的布尔值",
			zap.String("key", key), zap.String("value", value))
		return def
	}
	return parsed
}

// PublicValues 返回允许公开下发的配置值，keys 为空时返回全部公开项；
// 密钥类配置即使标记了公开也不下发
func PublicValues(keys []string) map[string]string {
	values := make(map[string]string)
	current := snapshot()

	if len(keys) == 0 {
		for key, item := range current {
			if item.IsPublic && !item.IsSecret {
				values[key] = item.Value
			}
		}
		return values
	}

	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		item, ok := current[key]
		if !ok || !item.IsPublic || item.IsSecret {
			continue
		}
		values[key] = item.Value
	}
	return values
}

// load 读取单个配置项
func load(key string) (Item, bool) {
	item, ok := snapshot()[key]
	return item, ok
}

// snapshot 返回当前配置缓存，未加载过时返回空表
func snapshot() map[string]Item {
	current, _ := items.Load().(map[string]Item)
	if current == nil {
		return map[string]Item{}
	}
	return current
}
