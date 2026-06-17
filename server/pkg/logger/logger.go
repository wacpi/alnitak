package logger

import (
	"os"
	"strings"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"interastral-peace.com/alnitak/internal/global"
)

// InitLogger 初始化全局日志。配置项：
//
//	mode: dev 同时输出到文件和控制台；prod 只输出到文件。
//	level: debug/info/warn/error，默认 info。
func InitLogger() (err error) {
	mode := global.Config.Log.Mode
	filename := global.Config.Log.FileName
	maxSize := global.Config.Log.MaxSize
	maxAge := global.Config.Log.MaxAge
	maxBackups := global.Config.Log.MaxBackups
	level := parseLevel(global.Config.Log.Level)

	writeSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
	})
	encoder := newJSONEncoder()

	var core zapcore.Core
	if mode == "dev" {
		consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
		core = zapcore.NewTee(
			zapcore.NewCore(encoder, writeSyncer, level),
			zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stdout), level),
		)
	} else {
		// prod: 所有级别写文件，ERROR+ 额外输出到 stderr（供 nssm/docker 捕获）
		core = zapcore.NewTee(
			zapcore.NewCore(encoder, writeSyncer, level),
			zapcore.NewCore(encoder, zapcore.Lock(os.Stderr), zapcore.ErrorLevel),
		)
	}

	log := zap.New(core, zap.AddCaller())
	zap.ReplaceGlobals(log)
	return
}

func newJSONEncoder() zapcore.Encoder {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.TimeKey = "time"
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	cfg.EncodeCaller = zapcore.ShortCallerEncoder
	return zapcore.NewJSONEncoder(cfg)
}

// parseLevel 把配置字符串转成 zapcore.Level，非法值回退到 InfoLevel。
func parseLevel(s string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
