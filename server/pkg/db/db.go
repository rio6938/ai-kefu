package db

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"admin/server/pkg/config"
	"admin/server/pkg/log"
)

// DB 是全局 gorm 实例，Init 之后可用
var DB *gorm.DB

// Init 连接 PostgreSQL 并设置连接池，sqlLevel 是 SQL 日志的级别
func Init(cfg config.DB, sqlLevel string) error {
	if cfg.DSN == "" {
		return fmt.Errorf("db dsn is empty")
	}

	d, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger: gormlogger.New(zapWriter{}, gormlogger.Config{
			SlowThreshold: 200 * time.Millisecond,
			LogLevel:      gormLogLevel(sqlLevel),
			Colorful:      false,
		}),
	})
	if err != nil {
		return err
	}

	sqlDB, err := d.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.MaxLifetime) * time.Second)

	DB = d
	return nil
}

// Close 关闭底层连接池
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// zapWriter 把 gorm 的日志转给 SQL 日志通道，只写日志文件
type zapWriter struct{}

func (zapWriter) Printf(format string, args ...any) {
	if log.SQL == nil {
		return
	}
	log.SQL.Debug(fmt.Sprintf(format, args...))
}

func gormLogLevel(level string) gormlogger.LogLevel {
	switch level {
	case "debug", "info":
		return gormlogger.Info
	case "warn":
		return gormlogger.Warn
	case "error":
		return gormlogger.Error
	case "silent", "off":
		return gormlogger.Silent
	default:
		return gormlogger.Warn
	}
}
