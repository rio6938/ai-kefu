package log

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"admin/server/pkg/config"
)

// Logger 是默认日志通道，全量写入日志文件，控制台只保留告警和错误；
// 未调用 Init 时是空实现，避免初始化顺序靠前的基础包写日志时 panic
var Logger = zap.NewNop()

// Access 是请求访问日志通道，同时输出到控制台和日志文件
var Access = zap.NewNop()

// Kernel 是启动、关闭等关键运行日志通道，同时输出到控制台和日志文件
var Kernel = zap.NewNop()

// SQL 是数据库日志通道，只写入日志文件
var SQL = zap.NewNop()

// sinks 是各通道的日志文件，退出时需要刷新关闭
var sinks *fileSinks

// Init 按配置初始化全局 logger，path 为空时不写文件
func Init(cfg config.Log) error {
	fileLevel, err := parseLevel(cfg.Level, "log level")
	if err != nil {
		return err
	}
	consoleLevel, err := parseLevel(cfg.ConsoleLevel, "console log level")
	if err != nil {
		return err
	}
	sqlLevel, err := parseLevel(cfg.SQLLevel, "sql log level")
	if err != nil {
		return err
	}

	encoderConfig := newEncoderConfig()
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
	consoleWriter := zapcore.Lock(os.Stdout)

	// 控制台 core，各通道使用各自的级别
	consoleCore := func(level zapcore.LevelEnabler) zapcore.Core {
		return zapcore.NewCore(consoleEncoder, consoleWriter, level)
	}

	sinks = newFileSinks(cfg)
	var serverFile, accessFile, sqlFile zapcore.Core
	if sinks != nil {
		serverFile = zapcore.NewCore(fileEncoder, zapcore.AddSync(sinks.server), fileLevel)
		accessFile = zapcore.NewCore(fileEncoder, zapcore.AddSync(sinks.access), fileLevel)
		sqlFile = zapcore.NewCore(fileEncoder, zapcore.AddSync(sinks.sql), sqlLevel)
	}

	// 默认通道的告警和错误必须能在控制台看到，其余级别只写文件
	Logger = newLogger(tee(serverFile, consoleCore(zapcore.WarnLevel)))
	Access = newLogger(tee(accessFile, consoleCore(consoleLevel)))
	Kernel = newLogger(tee(serverFile, consoleCore(consoleLevel)))
	SQL = newLogger(tee(sqlFile))

	zap.ReplaceGlobals(Logger)
	return nil
}

// Close 刷新日志并关闭日志文件，退出前调用
func Close() {
	for _, logger := range []*zap.Logger{Logger, Access, Kernel, SQL} {
		if logger != nil {
			// 终端输出不支持 Sync，返回的错误忽略即可
			_ = logger.Sync()
		}
	}
	if sinks != nil {
		_ = sinks.close()
	}
}

// GinWriter 返回 gin 调试输出的写入器，转成默认通道的 Debug 日志
func GinWriter() io.Writer {
	return ginLogWriter{level: zapcore.DebugLevel}
}

// GinErrorWriter 返回 gin 错误输出的写入器，转成默认通道的 Error 日志
func GinErrorWriter() io.Writer {
	return ginLogWriter{level: zapcore.ErrorLevel}
}

// ginLogWriter 把 gin 的文本输出转成 zap 日志
type ginLogWriter struct {
	level zapcore.Level
}

func (w ginLogWriter) Write(p []byte) (int, error) {
	message := strings.TrimSpace(string(p))
	if message != "" && Logger != nil {
		Logger.Log(w.level, "gin", zap.String("output", message))
	}
	return len(p), nil
}

// newLogger 包装 core，附加调用位置和错误栈信息
func newLogger(core zapcore.Core) *zap.Logger {
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

// newEncoderConfig 返回控制台与文件共用的编码配置
func newEncoderConfig() zapcore.EncoderConfig {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	cfg.EncodeDuration = zapcore.MillisDurationEncoder
	cfg.EncodeCaller = zapcore.ShortCallerEncoder
	return cfg
}

// tee 组合多个 core，忽略为空的 core
func tee(cores ...zapcore.Core) zapcore.Core {
	list := make([]zapcore.Core, 0, len(cores))
	for _, core := range cores {
		if core != nil {
			list = append(list, core)
		}
	}
	if len(list) == 0 {
		return zapcore.NewNopCore()
	}
	return zapcore.NewTee(list...)
}

// parseLevel 解析日志级别，silent 或 off 表示关闭该通道
func parseLevel(value, field string) (zapcore.LevelEnabler, error) {
	switch value {
	case "":
		return zapcore.InfoLevel, nil
	case "silent", "off":
		return zapcore.Level(zapcore.FatalLevel + 1), nil
	}
	level, err := zapcore.ParseLevel(value)
	if err != nil {
		return nil, fmt.Errorf("invalid %s %q: %w", field, value, err)
	}
	return level, nil
}

// fileSinks 是各通道的日志文件
type fileSinks struct {
	server *lumberjack.Logger
	access *lumberjack.Logger
	sql    *lumberjack.Logger
	done   chan struct{}  // 关闭时通知按天切割任务退出
	wg     sync.WaitGroup // 等待按天切割任务结束，避免关闭文件后再被切割
}

// newFileSinks 按配置创建各通道的日志文件，日志目录不可写时返回 nil
func newFileSinks(cfg config.Log) *fileSinks {
	if cfg.Path == "" {
		return nil
	}
	if err := os.MkdirAll(cfg.Path, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "log: create dir %q: %v, only output to console\n", cfg.Path, err)
		return nil
	}
	sinks := &fileSinks{
		server: newFileWriter(cfg, "server"),
		access: newFileWriter(cfg, "access"),
		sql:    newFileWriter(cfg, "sql"),
		done:   make(chan struct{}),
	}
	sinks.wg.Add(1)
	go sinks.rotateDaily()
	return sinks
}

// newFileWriter 创建按大小和保留天数切割的日志文件
func newFileWriter(cfg config.Log, name string) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:  filepath.Join(cfg.Path, name+".log"),
		MaxSize:   cfg.MaxSize,
		MaxAge:    cfg.MaxAge,
		LocalTime: true,
	}
}

// rotateDaily 每天本地零点切割各通道日志文件，close 后退出
func (s *fileSinks) rotateDaily() {
	defer s.wg.Done()
	for {
		timer := time.NewTimer(untilMidnight())
		select {
		case <-timer.C:
			s.rotate()
		case <-s.done:
			timer.Stop()
			return
		}
	}
}

// untilMidnight 返回距离下一个本地零点的时长
func untilMidnight() time.Duration {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return time.Until(next)
}

// rotate 切割三个通道的日志文件，单个通道失败不影响其余通道
func (s *fileSinks) rotate() {
	for _, file := range []struct {
		name   string
		logger *lumberjack.Logger
	}{
		{"server", s.server},
		{"access", s.access},
		{"sql", s.sql},
	} {
		if err := file.logger.Rotate(); err != nil {
			fmt.Fprintf(os.Stderr, "log: rotate %s log: %v\n", file.name, err)
		}
	}
}

// close 停止按天切割任务并关闭所有日志文件
func (s *fileSinks) close() error {
	close(s.done)
	s.wg.Wait()
	return errors.Join(s.server.Close(), s.access.Close(), s.sql.Close())
}
