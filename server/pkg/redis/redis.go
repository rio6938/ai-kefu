package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"admin/server/pkg/config"
)

// Client 是全局 redis 客户端，Init 之后可用
var Client *goredis.Client

// Init 连接 redis 并做一次探活
func Init(cfg config.Redis) error {
	if cfg.Addr == "" {
		return fmt.Errorf("redis addr is empty")
	}

	c := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		return err
	}

	Client = c
	return nil
}

// Close 关闭 redis 连接
func Close() error {
	if Client == nil {
		return nil
	}
	return Client.Close()
}
