package logger

import (
	"github.com/godyy/glog"
)

type Logger = glog.Logger

// Config 日志配置
type Config struct {
	// Level 日志等级
	Level glog.Level

	// Caller 是否记录调用者
	Caller bool

	// CallerSkip 调用者跳过层数
	CallerSkip int

	// Development 是否开发模式
	Development bool

	// EnableStd 是否启用标准输出
	EnableStd bool

	// FileParams 日志文件输出相关 Core 配置参数
	FileParams *glog.FileCoreParams
}

// CreateLogger 创建日志实例.
func CreateLogger(cfg *Config) Logger {
	glogCfg := &glog.Config{
		Level:        cfg.Level,
		EnableCaller: cfg.Caller,
		CallerSkip:   cfg.CallerSkip,
		Development:  cfg.Development,
	}
	if cfg.EnableStd {
		glogCfg.Cores = append(glogCfg.Cores, glog.NewStdCoreConfig())
	}
	if cfg.FileParams != nil {
		glogCfg.Cores = append(glogCfg.Cores, glog.NewFileCoreConfig(cfg.FileParams))
	}
	return glog.NewLogger(glogCfg)
}
