package flags

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// Value 表示flag值的类型约束.
type Value interface {
	int | int64 | uint | uint64 | float64 | time.Duration | bool | string
}

var (
	// flagSet 表示flag集.
	flagSet *flag.FlagSet

	// valueMap 存储flag值的map.
	valueMap = map[string]any{}

	// parsedFuncs 存储解析完成后的回调函数.
	parsedFuncs []func()
)

// getFlagSet 获取flag集.
func getFlagSet() *flag.FlagSet {
	if flagSet == nil {
		if len(os.Args) == 0 {
			flagSet = flag.NewFlagSet("", flag.ExitOnError)
		} else {
			flagSet = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		}
	}
	return flagSet
}

// addValue 添加flag值.
func addValue[val Value](name string, p *val) {
	if valueMap == nil {
		valueMap = make(map[string]any)
	}
	if _, ok := valueMap[name]; ok {
		panic(fmt.Sprintf("flag name %s already exists", name))
	}
	valueMap[name] = p
}

// Parse 解析flag.
func Parse() {
	if flagSet == nil || flagSet.Parsed() {
		return
	}

	flagSet.Parse(os.Args[1:])

	// 调用解析完成后的回调函数.
	for _, f := range parsedFuncs {
		f()
	}
}

// Reset 重置, 清除所有状态.
func Reset() {
	flagSet = nil
	valueMap = nil
	parsedFuncs = nil
}

// Int 设置int类型flag.
func Int(name string, value int, usage string) *int {
	pv := getFlagSet().Int(name, value, usage)
	addValue(name, pv)
	return pv
}

// Int64 设置int64类型flag.
func Int64(name string, value int64, usage string) *int64 {
	pv := getFlagSet().Int64(name, value, usage)
	addValue(name, pv)
	return pv
}

// Uint 设置uint类型flag.
func Uint(name string, value uint, usage string) *uint {
	pv := getFlagSet().Uint(name, value, usage)
	addValue(name, pv)
	return pv
}

// Uint64 设置uint64类型flag.
func Uint64(name string, value uint64, usage string) *uint64 {
	pv := getFlagSet().Uint64(name, value, usage)
	addValue(name, pv)
	return pv
}

// Float64 设置float64类型flag.
func Float64(name string, value float64, usage string) *float64 {
	pv := getFlagSet().Float64(name, value, usage)
	addValue(name, pv)
	return pv
}

// Duration 设置time.Duration类型flag.
func Duration(name string, value time.Duration, usage string) *time.Duration {
	pv := getFlagSet().Duration(name, value, usage)
	addValue(name, pv)
	return pv
}

// Bool 设置bool类型flag.
func Bool(name string, value bool, usage string) *bool {
	pv := getFlagSet().Bool(name, value, usage)
	addValue(name, pv)
	return pv
}

// String 设置string类型flag.
func String(name string, value string, usage string) *string {
	pv := getFlagSet().String(name, value, usage)
	addValue(name, pv)
	return pv
}

// AddFlag 添加flag.
func AddFlag[val Value](name string, value val, usage string) *val {
	var i any = value
	switch o := i.(type) {
	case int:
		i = Int(name, o, usage)
	case int64:
		i = Int64(name, o, usage)
	case uint:
		i = Uint(name, o, usage)
	case uint64:
		i = Uint64(name, o, usage)
	case float64:
		i = Float64(name, o, usage)
	case time.Duration:
		i = Duration(name, o, usage)
	case bool:
		i = Bool(name, o, usage)
	case string:
		i = String(name, o, usage)
	default:
		panic("invalid flag value type")
	}
	return i.(*val)
}

// GetValue 获取flag值.
func GetValue[val Value](name string) (v val, exist bool) {
	if pv := valueMap[name]; pv == nil {
		exist = false
	} else {
		v = *(pv.(*val))
		exist = true
	}
	return
}

// AddParsedFunc 添加解析完成后的回调函数.
func AddParsedFunc(f func()) {
	parsedFuncs = append(parsedFuncs, f)
}
