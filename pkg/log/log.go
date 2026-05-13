package log

import (
	"context"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	prettyconsole "github.com/thessem/zap-prettyconsole"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type ctxLoggerKey struct{}

// Logger 是 zap.Logger 的项目包装：通过 WithValue/WithContext 把请求级
// trace/user 等字段挂到 ctx，让下游各层无须显式传 logger 即可复用同一上下文。
type Logger struct {
	*zap.Logger
}

// NewLog 按 viper 配置构造按 level/encoding 分流的 Logger。
//
// 设计要点：stdout 与 lumberjack 文件走两个独立 core（zapcore.NewTee）。
//   - 控制台路径：encoding=console 时使用 prettyconsole，自动对齐列宽、彩色、
//     宽字符（中文）按显示宽度计算。
//   - 文件路径：始终使用 JSON 编码，避免 ANSI 颜色控制字符污染日志文件，
//     也便于 ELK/Loki 等管道消费结构化日志。
//   - encoding=json 时控制台也回退到 JSON，prod 场景统一结构化输出。
func NewLog(conf *viper.Viper) *Logger {
	level := parseLevel(conf.GetString("log.log_level"))
	hook := lumberjack.Logger{
		Filename:   conf.GetString("log.log_file_name"),
		MaxSize:    conf.GetInt("log.max_size"),
		MaxBackups: conf.GetInt("log.max_backups"),
		MaxAge:     conf.GetInt("log.max_age"),
		Compress:   conf.GetBool("log.compress"),
	}

	jsonEnc := zapcore.NewJSONEncoder(jsonEncoderConfig())
	fileCore := zapcore.NewCore(jsonEnc, zapcore.AddSync(&hook), level)

	var stdoutCore zapcore.Core
	if conf.GetString("log.encoding") == "console" {
		stdoutCore = zapcore.NewCore(
			prettyconsole.NewEncoder(prettyconsole.NewEncoderConfig()),
			zapcore.AddSync(os.Stdout),
			level,
		)
	} else {
		// 复用同一份 JSON 编码器即可；不同 core 间 zap 内部已做并发安全处理。
		stdoutCore = zapcore.NewCore(jsonEnc, zapcore.AddSync(os.Stdout), level)
	}

	core := zapcore.NewTee(stdoutCore, fileCore)
	opts := []zap.Option{zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel)}
	if conf.GetString("env") != "prod" {
		opts = append(opts, zap.Development())
	}
	return &Logger{zap.New(core, opts...)}
}

// parseLevel 将配置字符串映射为 zap 级别；未识别值回退到 info，避免拼写错误
// 导致整个进程默认到 debug 把日志刷爆，也避免直接 panic 阻塞启动。
func parseLevel(lv string) zapcore.Level {
	switch lv {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

func jsonEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// WithValue 将字段写入请求上下文，保证后续日志自动携带 trace、用户等关联信息。
func (l *Logger) WithValue(ctx context.Context, fields ...zapcore.Field) context.Context {
	if c, ok := ctx.(*gin.Context); ok {
		ctx = c.Request.Context()
		c.Request = c.Request.WithContext(context.WithValue(ctx, ctxLoggerKey{}, l.WithContext(ctx).With(fields...)))
		return c
	}
	return context.WithValue(ctx, ctxLoggerKey{}, l.WithContext(ctx).With(fields...))
}

// WithContext 优先返回上下文 logger；缺失时回退到全局 logger，避免调用方做空值分支。
func (l *Logger) WithContext(ctx context.Context) *Logger {
	if c, ok := ctx.(*gin.Context); ok {
		ctx = c.Request.Context()
	}
	zl := ctx.Value(ctxLoggerKey{})
	ctxLogger, ok := zl.(*zap.Logger)
	if ok {
		return &Logger{ctxLogger}
	}
	return l
}
