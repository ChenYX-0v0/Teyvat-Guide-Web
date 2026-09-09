package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var global *zap.SugaredLogger

// Init 初始化全局日志器
func Init(level, mode string) {
	lv, err := zapcore.ParseLevel(level)
	if err != nil {
		lv = zapcore.InfoLevel
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(lv)
	if mode == "console" {
		cfg = zap.NewDevelopmentConfig()
		cfg.Level = zap.NewAtomicLevelAt(lv)
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	l, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	global = l.Sugar()
}

// L 获取全局 SugaredLogger（未初始化时返回 noop），支持 Infow / Errorw / Fatalw
func L() *zap.SugaredLogger {
	if global == nil {
		global = zap.NewNop().Sugar()
	}
	return global
}

// Sync 刷新缓冲区
func Sync() error {
	return L().Sync()
}
