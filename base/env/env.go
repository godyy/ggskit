package env

const (
	StageDev  = "dev"  // 开发环境
	StageProd = "prod" // 生产环境
)

// Env 环境变量管理器.
type Env interface {
	// Stage 返回当前环境.
	Stage() string

	// Dev 返回是否为开发环境.
	Dev() bool

	// Prod 返回是否为生产环境.
	Prod() bool

	// Debug 返回是否启用调试模式.
	Debug() bool
}

var env = &envImpl{
	stage: StageDev,
}

// Get 返回环境变量管理器.
func Get() Env {
	return env
}

// envImpl 环境变量管理器实现.
type envImpl struct {
	stage string // 环境
	debug bool   // 是否启用调试模式
}

func (env *envImpl) Stage() string {
	return env.stage
}

func (env *envImpl) Dev() bool {
	return env.stage == StageDev
}

func (env *envImpl) Prod() bool {
	return env.stage == StageProd
}

func (env *envImpl) Debug() bool {
	return env.debug && !env.Prod()
}
