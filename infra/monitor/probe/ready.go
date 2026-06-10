package probe

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"time"
)

// ErrDuplicateReadinessChecker 重复注册同名就绪检查器。
var ErrDuplicateReadinessChecker = errors.New("duplicate readiness checker")

// ReadinessPolicy 就绪策略
type ReadinessPolicy int

const (
	AlwaysCheck ReadinessPolicy = iota
	GateOnly                    // 仅依赖 SetReady 门闩，不执行检查器
	Cached                      // 周期性执行检查器并缓存结果
)

// ReadinessChecker 就绪检查器
type ReadinessChecker func(ctx context.Context) error

// readinessCache 就绪检查缓存
type readinessCache struct {
	ready bool              // 状态.
	fails map[string]string // 失败检查器及原因.
	ts    time.Time         // 缓存时间.
}

var (
	readyFlag         atomic.Bool                                         // 就绪门闩
	readinessCheckers                 = make(map[string]ReadinessChecker) // 就绪检查器
	readinessPolicy   ReadinessPolicy = AlwaysCheck                       // 继续策略
	readinessTimeout                  = 5 * time.Second                   // 就绪检查超时时间
	readinessCacheTTL                 = 2 * time.Second                   // Cached 策略下的检查结果缓存 TTL
	readinessCached   readinessCache                                      // Cached 策略下的检查结果缓存
)

// SetReady 运行期就绪门闩（允许切换）
func SetReady(ready bool) {
	readyFlag.Store(ready)
}

// readyzHandler 原生 http 就绪探针.
func readyzHandler() http.HandlerFunc {
	checkInitialized(true)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 门闩未放开直接 503
		if !readyFlag.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "not-ready",
				"reason": "manual_gate_not_open",
			})
			return
		}

		switch readinessPolicy {
		case GateOnly:
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ready"})
			return
		case Cached:
			if time.Since(readinessCached.ts) < readinessCacheTTL {
				if !readinessCached.ready {
					w.WriteHeader(http.StatusServiceUnavailable)
					_ = json.NewEncoder(w).Encode(map[string]any{"status": "not-ready", "fails": readinessCached.fails})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "ready"})
				return
			}
			ready, fails := runChecks(r.Context())
			readinessCached.ready, readinessCached.fails, readinessCached.ts = ready, fails, time.Now()
			if !ready {
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "not-ready", "fails": fails})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ready"})
			return
		default:
			ready, fails := runChecks(r.Context())
			if !ready {
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "not-ready", "fails": fails})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ready"})
		}

	}
}

// 并发执行就绪检查器，并带默认超时。
func runChecks(parent context.Context) (bool, map[string]string) {
	type result struct {
		name string
		err  error
	}

	checkers := make([]struct {
		name string
		fn   ReadinessChecker
	}, 0, len(readinessCheckers))
	for k, v := range readinessCheckers {
		checkers = append(checkers, struct {
			name string
			fn   ReadinessChecker
		}{name: k, fn: v})
	}

	ctx, cancel := context.WithTimeout(parent, readinessTimeout)
	defer cancel()

	resCh := make(chan result, len(checkers))
	for _, c := range checkers {
		go func(c struct {
			name string
			fn   ReadinessChecker
		}) {
			resCh <- result{name: c.name, err: c.fn(ctx)}
		}(c)
	}

	fails := make(map[string]string)
	for i := 0; i < len(checkers); i++ {
		res := <-resCh
		if res.err != nil {
			fails[res.name] = res.err.Error()
		}
	}
	return len(fails) == 0, fails
}
