package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New собирает логгер под окружение: локально — читаемый вывод с debug,
// в остальных случаях — JSON уровня info.
func New(env string) *zap.Logger {
	var cfg zap.Config

	switch env {
	case "local", "dev":
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	default:
		cfg = zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return zap.Must(cfg.Build())
}
