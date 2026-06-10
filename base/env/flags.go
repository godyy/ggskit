package env

import (
	"github.com/godyy/ggskit/base/flags"
)

// FlagName 返回flag选项名.
func FlagName(name string) string {
	return "env-" + name
}

// AddFlag 添加flag选项.
func AddFlag[T flags.Value](name string, value T, usage string) *T {
	return flags.AddFlag(FlagName(name), value, usage)
}

// GetFlagValue 获取flag选项值.
func GetFlagValue[T flags.Value](name string) (T, bool) {
	return flags.GetValue[T](FlagName(name))
}

func (env *envImpl) applyFlags() {
	env.stage, _ = GetFlagValue[string]("stage")
	env.debug, _ = GetFlagValue[bool]("debug")
}

func init() {
	AddFlag("stage", StageDev, "stage")
	AddFlag("debug", false, "debug")
	flags.AddParsedFunc(env.applyFlags)
}
