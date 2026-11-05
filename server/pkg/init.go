package pkg

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"admin/server/internal"
	"admin/server/pkg/captcha"
	"admin/server/pkg/config"
	"admin/server/pkg/db"
	"admin/server/pkg/encrypt"
	"admin/server/pkg/jwt"
	"admin/server/pkg/log"
	redisc "admin/server/pkg/redis"
	"admin/server/pkg/server"
	"admin/server/pkg/sms"
)

// Config 是加载后的全局配置
var Config *config.Config

// Init 按 配置 → 日志 → 数据库 → redis → jwt/验证码/加密 → 迁移 → 加载配置中心 → 短信 → 路由 的顺序完成装配
func Init(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config %q: %w", configPath, err)
	}
	Config = cfg

	if err := log.Init(cfg.Log); err != nil {
		return fmt.Errorf("init log: %w", err)
	}

	if err := db.Init(cfg.DB, cfg.Log.SQLLevel); err != nil {
		return fmt.Errorf("init db: %w", err)
	}

	if err := redisc.Init(cfg.Redis); err != nil {
		return fmt.Errorf("init redis: %w", err)
	}

	jwt.Init(cfg.JWT)
	jwt.InitUser(cfg.JWT)
	captcha.Init(cfg.Captcha, cfg.Server.Mode == "debug", redisc.Client)
	if err := encrypt.Init(cfg.Security); err != nil {
		return fmt.Errorf("init encrypt: %w", err)
	}

	if err := internal.Migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// 配置中心的配置必须先加载，依赖配置的客户端在初始化时直接读缓存
	if err := internal.LoadConfigCache(); err != nil {
		return fmt.Errorf("load config cache: %w", err)
	}

	// 客服模块依赖配置的客户端：IM、向量库、跨域
	if err := internal.InitKefu(cfg); err != nil {
		return fmt.Errorf("init kefu: %w", err)
	}

	if err := sms.Init(cfg.Server.Mode == "debug", redisc.Client); err != nil {
		return fmt.Errorf("init sms: %w", err)
	}

	internal.Register(server.Init(cfg.Server.Mode))

	log.Kernel.Info("init done",
		zap.String("config", configPath),
		zap.String("mode", cfg.Server.Mode),
		zap.String("addr", cfg.Server.Addr),
	)
	return nil
}

// Run 启动 HTTP 服务，收到 SIGINT/SIGTERM 后优雅退出
func Run() error {
	defer log.Close()

	srv := &http.Server{
		Addr:    Config.Server.Addr,
		Handler: server.Engine,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	log.Kernel.Info("http server listening", zap.String("addr", Config.Server.Addr))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("listen: %w", err)
	case <-quit:
	}

	log.Kernel.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := redisc.Close(); err != nil {
		return fmt.Errorf("close redis: %w", err)
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("close db: %w", err)
	}
	log.Kernel.Info("bye")
	return nil
}
