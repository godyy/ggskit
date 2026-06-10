package config

import (
	"github.com/godyy/ggskit/base/flags"
)

// ConfigWithFlags 实现该接口, 表示在配置数据解码后, 需要应用flag选项值.
type ConfigWithFlags interface {
	// ApplyFlags 应用 flag 选项
	ApplyFlags() error
}

// FlagName 返回flag选项名.
func FlagName(name string) string {
	return "conf-" + name
}

// AddFlag 添加flag选项.
func AddFlag[T flags.Value](name string, value T, usage string) *T {
	return flags.AddFlag(FlagName(name), value, usage)
}

// GetFlagValue 获取flag选项值.
func GetFlagValue[T flags.Value](name string) (T, bool) {
	return flags.GetValue[T](FlagName(name))
}
