package database

import "time"

type Options struct {
	MaxOpenConnections int
	MaxIdleConnections int
	ConnMaxLifetime    time.Duration
	ConnMaxIdleTime    time.Duration
	LogLevel           LogLevel
	SlowThreshold      time.Duration
}

type LogLevel int

const (
	LogSilent LogLevel = iota
	LogError
	LogWarn
	LogInfo
)

type Option func(*Options)

func defaultOptions() *Options {
	return &Options{
		MaxOpenConnections: 25,
		MaxIdleConnections: 5,
		ConnMaxLifetime:    5 * time.Minute,
		ConnMaxIdleTime:    5 * time.Minute,
		LogLevel:           LogError,
		SlowThreshold:      200 * time.Millisecond,
	}
}
