package probe

import (
	"time"
)

// Option 初始化期配置项，仅在 Init/MustInit 中使用。
type Option func()

// WithReadinessChecker 注册一个就绪检查器。
func WithReadinessChecker(name string, checker ReadinessChecker) Option {
	return func() {
		checkInitialized(false)
		if _, exists := readinessCheckers[name]; exists {
			panic(ErrDuplicateReadinessChecker)
		}
		readinessCheckers[name] = checker
	}
}

// WithReadinessCheckers 批量注册就绪检查器。
func WithReadinessCheckers(m map[string]ReadinessChecker) Option {
	return func() {
		checkInitialized(false)
		for name, checker := range m {
			if _, exists := readinessCheckers[name]; exists {
				panic(ErrDuplicateReadinessChecker)
			}
			readinessCheckers[name] = checker
		}
	}
}

// WithReadinessPolicy 设置就绪策略：AlwaysCheck/GateOnly/Cached。
func WithReadinessPolicy(p ReadinessPolicy) Option {
	return func() {
		checkInitialized(false)
		readinessPolicy = p
	}
}

// WithReadinessCacheTTL 设置 Cached 策略下的检查结果缓存 TTL。
func WithReadinessCacheTTL(ttl time.Duration) Option {
	return func() {
		checkInitialized(false)
		readinessCacheTTL = ttl
	}
}

// WithReadinessTimeout 设置每次就绪检查的默认超时。
func WithReadinessTimeout(d time.Duration) Option {
	return func() {
		checkInitialized(false)
		readinessTimeout = d
	}
}
