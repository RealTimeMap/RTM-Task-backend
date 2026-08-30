package database

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm/logger"
)

type gormLogger struct {
	log           *zap.Logger
	logLevel      logger.LogLevel
	slowThreshold time.Duration
}

func newGormLogger(log *zap.Logger, level LogLevel, slowThreshold time.Duration) logger.Interface {
	var gormLevel logger.LogLevel
	switch level {
	case LogSilent:
		gormLevel = logger.Silent
	case LogError:
		gormLevel = logger.Error
	case LogWarn:
		gormLevel = logger.Warn
	case LogInfo:
		gormLevel = logger.Info
	default:
		gormLevel = logger.Error
	}

	return &gormLogger{
		log:           log.Named("gorm"),
		logLevel:      gormLevel,
		slowThreshold: slowThreshold,
	}
}

func (l *gormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.logLevel = level
	return &newLogger
}

func (l *gormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= logger.Info {
		l.log.Sugar().Infof(msg, data...)
	}
}

func (l *gormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= logger.Warn {
		l.log.Sugar().Warnf(msg, data...)
	}
}

func (l *gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= logger.Error {
		l.log.Sugar().Errorf(msg, data...)
	}
}

func (l *gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.logLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	fields := []zap.Field{
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rows),
		zap.String("sql", sql),
	}

	switch {
	case err != nil && !errors.Is(err, logger.ErrRecordNotFound):
		l.log.Error("database error", append(fields, zap.Error(err))...)

	case elapsed > l.slowThreshold:
		l.log.Warn("slow query", fields...)

	case l.logLevel >= logger.Info:
		l.log.Debug("query", fields...)
	}
}
