package zapgorm2

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
	gormlogger "gorm.io/gorm/logger"
)

const ctxLoggerKey = "zapLogger"

// Logger 把 GORM logger 协议适配到 zap：实现 GORM 的 Info/Warn/Error/Trace，
// 同时优先复用 ctx 上挂的请求级 zap.Logger（在中间件里通过 WithValue 设置），
// 保证 SQL 日志能携带 trace/用户等关联字段。
type Logger struct {
	ZapLogger                 *zap.Logger
	SlowThreshold             time.Duration
	Colorful                  bool
	IgnoreRecordNotFoundError bool
	ParameterizedQueries      bool
	LogLevel                  gormlogger.LogLevel
}

// New 构造默认 GORM logger：Warn 级、100ms 慢查询阈值，
// 与 GORM 官方默认行为对齐，避免开发期被 Info 级 SQL 淹没。
func New(zapLogger *zap.Logger) gormlogger.Interface {
	return &Logger{
		ZapLogger:                 zapLogger,
		LogLevel:                  gormlogger.Warn,
		SlowThreshold:             100 * time.Millisecond,
		Colorful:                  false,
		IgnoreRecordNotFoundError: false,
		ParameterizedQueries:      false,
	}
}

// LogMode 按 GORM logger 协议返回一个携带新 level 的副本，
// 保证全局 logger 不被并发场景下的局部调整污染。
func (l *Logger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newlogger := *l
	newlogger.LogLevel = level
	return &newlogger
}

// Info 把 gorm 内部的 Info 级日志桥接到 ctx 派生的 zap.Logger（含 trace/请求字段）。
func (l Logger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Info {
		l.logger(ctx).Sugar().Infof(msg, data...)
	}
}

// Warn 同 Info，按 gorm logger 协议处理 Warn 级日志。
func (l Logger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Warn {
		l.logger(ctx).Sugar().Warnf(msg, data...)
	}
}

// Error 同 Info，按 gorm logger 协议处理 Error 级日志。
func (l Logger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Error {
		l.logger(ctx).Sugar().Errorf(msg, data...)
	}
}

// Trace 是 GORM 每条 SQL 执行后的回调；按 elapsed / err / 慢阈值
// 决定落到哪一级，并附带 sql/rows/elapsed 结构化字段。
func (l Logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	elapsedStr := fmt.Sprintf("%.3fms", float64(elapsed.Nanoseconds())/1e6)
	logger := l.logger(ctx)
	switch {
	case err != nil && l.LogLevel >= gormlogger.Error && (!errors.Is(err, gormlogger.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		if rows == -1 {
			logger.Error("trace", zap.Error(err), zap.String("elapsed", elapsedStr), zap.Int64("rows", rows), zap.String("sql", sql))
		} else {
			logger.Error("trace", zap.Error(err), zap.String("elapsed", elapsedStr), zap.Int64("rows", rows), zap.String("sql", sql))
		}
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= gormlogger.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		if rows == -1 {
			logger.Warn("trace", zap.String("slow", slowLog), zap.String("elapsed", elapsedStr), zap.Int64("rows", rows), zap.String("sql", sql))
		} else {
			logger.Warn("trace", zap.String("slow", slowLog), zap.String("elapsed", elapsedStr), zap.Int64("rows", rows), zap.String("sql", sql))
		}
	case l.LogLevel == gormlogger.Info:
		sql, rows := fc()
		if rows == -1 {
			logger.Info("trace", zap.String("elapsed", elapsedStr), zap.Int64("rows", rows), zap.String("sql", sql))
		} else {
			logger.Info("trace", zap.String("elapsed", elapsedStr), zap.Int64("rows", rows), zap.String("sql", sql))
		}
	}
}

var (
	gormPackage = filepath.Join("gorm.io", "gorm")
)

func (l Logger) logger(ctx context.Context) *zap.Logger {
	logger := l.ZapLogger
	if ctx != nil {
		if c, ok := ctx.(*gin.Context); ok {
			ctx = c.Request.Context()
		}
		zl := ctx.Value(ctxLoggerKey)
		ctxLogger, ok := zl.(*zap.Logger)
		if ok {
			logger = ctxLogger
		}
	}

	for i := 2; i < 15; i++ {
		_, file, _, ok := runtime.Caller(i)
		switch {
		case !ok:
		case strings.HasSuffix(file, "_test.go"):
		case strings.Contains(file, gormPackage):
		default:
			return logger.WithOptions(zap.AddCallerSkip(i - 1))
		}
	}
	return logger
}
