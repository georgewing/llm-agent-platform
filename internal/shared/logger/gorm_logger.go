package logger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// GormZapLogger 实现 gorm.io/gorm/logger.Interface，将 SQL 日志桥接到 zap
type GormZapLogger struct {
	zapLogger     *zap.Logger
	level         gormlogger.LogLevel
	slowThreshold time.Duration
}

func NewGormZapLogger(zapLogger *zap.Logger, opts ...GormLoggerOption) gormlogger.Interface {
	gl := &GormZapLogger{
		zapLogger:     zapLogger.Named("gorm"), // 子 logger，日志中自动带 "logger":"gorm"
		level:         gormlogger.Warn,         // 默认只记录 Warn 及以上
		slowThreshold: 200 * time.Millisecond,  // 慢查询阈值
	}
	for _, opt := range opts {
		opt(gl)
	}
	return gl
}

// -- Option 模式 --

type GormLoggerOption func(*GormZapLogger)

func WithSlowThreshold(d time.Duration) GormLoggerOption {
	return func(gl *GormZapLogger) { gl.slowThreshold = d }
}

func WithGormLogLevel(level gormlogger.LogLevel) GormLoggerOption {
	return func(gl *GormZapLogger) { gl.level = level }
}

// -- 实现 gorm/logger.Interface 的 4 个方法 --

func (l *GormZapLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.level = level
	return &newLogger
}

func (l *GormZapLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= gormlogger.Info {
		l.zapLogger.Sugar().Infof(msg, args...)
	}
}

func (l *GormZapLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= gormlogger.Warn {
		l.zapLogger.Sugar().Warnf(msg, args...)
	}
}

func (l *GormZapLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= gormlogger.Error {
		l.zapLogger.Sugar().Errorf(msg, args...)
	}
}

func (l *GormZapLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	// 从 context 中提取 trace 信息（DDD 多租户 + 链路追踪的关键）
	fields := []zap.Field{
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rows),
		zap.String("sql", sql),
	}

	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		l.zapLogger.Error("sql error", append(fields, zap.Error(err))...)
	case elapsed > l.slowThreshold && l.slowThreshold > 0:
		l.zapLogger.Warn(fmt.Sprintf("slow sql (>%v)", l.slowThreshold), fields...)
	default:
		l.zapLogger.Debug("sql trace", fields...)
	}
}
