// Package metrics 基于 Prometheus 实现 gactor 层面的服务监控采集.
//
// 该包只负责把 gactor.ServiceMonitor 上报的数据转换为 Prometheus 指标,
// 指标暴露入口由 infra/monitor 侧提供.
package metrics

import (
	"strconv"
	"time"

	"github.com/godyy/gactor"
	pkgerrors "github.com/pkg/errors"
	prom "github.com/prometheus/client_golang/prometheus"
)

// 编译期校验对 gactor 监控接口的实现.
var (
	_ gactor.ServiceMonitor          = (*Metrics)(nil)
	_ gactor.ServiceMigrationMonitor = (*Metrics)(nil)
)

// DefaultNamespace 默认的指标命名空间.
const DefaultNamespace = "ggskit"

// migrationConvergedBuckets 迁移收敛耗时(秒)的分桶.
var migrationConvergedBuckets = []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300}

// Config 指定 Actor 指标采集器的配置.
type Config struct {
	// Registry 指标注册表; 为空时内部创建独立的注册表.
	Registry *prom.Registry

	// Namespace 指标命名空间, 作为所有指标名的前缀; 为空时使用 DefaultNamespace.
	Namespace string

	// CategoryLabeler 将 Actor 分类转换为指标 category 标签值; 为空时使用分类的十进制字符串.
	CategoryLabeler func(gactor.ActorCategory) string
}

// Metrics 实现 gactor.ServiceMonitor 与 gactor.ServiceMigrationMonitor.
type Metrics struct {
	registry *prom.Registry

	// actorAmount 当前存活的 Actor 数量.
	actorAmount *prom.GaugeVec
	// actorAction Actor 启动/停止异常等事件计数.
	actorAction *prom.CounterVec
	// timerAmount 当前存活的定时器数量.
	timerAmount prom.Gauge
	// timerAction 定时器事件计数.
	timerAction *prom.CounterVec
	// commAction RPC/Cast 通信事件计数.
	commAction *prom.CounterVec
	// migrationAction 迁移事件计数.
	migrationAction *prom.CounterVec
	// migrationConverged 迁移收敛耗时.
	migrationConverged prom.Histogram

	categoryLabeler func(gactor.ActorCategory) string
}

// New 创建 Actor 指标采集器.
func New(cfg *Config) (*Metrics, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	registry := cfg.Registry
	if registry == nil {
		registry = prom.NewRegistry()
	}

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = DefaultNamespace
	}

	labeler := cfg.CategoryLabeler
	if labeler == nil {
		labeler = func(category gactor.ActorCategory) string {
			return strconv.FormatUint(uint64(category), 10)
		}
	}

	m := &Metrics{
		registry:        registry,
		categoryLabeler: labeler,
		actorAmount: prom.NewGaugeVec(prom.GaugeOpts{
			Namespace: namespace,
			Name:      "actor_amount",
			Help:      "当前存活的 Actor 数量.",
		}, []string{"category"}),
		actorAction: prom.NewCounterVec(prom.CounterOpts{
			Namespace: namespace,
			Name:      "actor_action_total",
			Help:      "Actor 启动/停止异常等事件累计次数.",
		}, []string{"category", "action"}),
		timerAmount: prom.NewGauge(prom.GaugeOpts{
			Namespace: namespace,
			Name:      "timer_amount",
			Help:      "当前存活的定时器数量.",
		}),
		timerAction: prom.NewCounterVec(prom.CounterOpts{
			Namespace: namespace,
			Name:      "timer_action_total",
			Help:      "定时器事件累计次数.",
		}, []string{"action"}),
		commAction: prom.NewCounterVec(prom.CounterOpts{
			Namespace: namespace,
			Name:      "comm_action_total",
			Help:      "RPC/Cast 通信事件累计次数.",
		}, []string{"type", "action"}),
		migrationAction: prom.NewCounterVec(prom.CounterOpts{
			Namespace: namespace,
			Name:      "migration_action_total",
			Help:      "Actor 迁移事件累计次数.",
		}, []string{"action"}),
		migrationConverged: prom.NewHistogram(prom.HistogramOpts{
			Namespace: namespace,
			Name:      "migration_converged_seconds",
			Help:      "Actor 迁移收敛耗时(秒).",
			Buckets:   migrationConvergedBuckets,
		}),
	}

	collectors := []prom.Collector{
		m.actorAmount,
		m.actorAction,
		m.timerAmount,
		m.timerAction,
		m.commAction,
		m.migrationAction,
		m.migrationConverged,
	}
	for _, collector := range collectors {
		if err := registry.Register(collector); err != nil {
			return nil, pkgerrors.WithMessage(err, "register actor metrics")
		}
	}

	return m, nil
}

// Registry 返回指标注册表.
func (m *Metrics) Registry() *prom.Registry {
	return m.registry
}

// MonitorActorAmount 上报 Actor 数量变化.
// action 为启动时累加 amount, 为停止时扣减 amount, 其余动作按事件累计.
func (m *Metrics) MonitorActorAmount(action gactor.MonitorActorAction, category gactor.ActorCategory, amount int64) {
	label := m.categoryLabeler(category)
	switch action {
	case gactor.MonitorActorStart:
		m.actorAmount.WithLabelValues(label).Add(float64(amount))
	case gactor.MonitorActorStop:
		m.actorAmount.WithLabelValues(label).Sub(float64(amount))
	default:
		m.actorAction.WithLabelValues(label, action.MonitorKey()).Add(float64(amount))
	}
}

// MonitorTimerAmount 上报定时器数量变化.
// action 为启动时累加 amount, 为停止时扣减 amount, 触发按事件累计.
func (m *Metrics) MonitorTimerAmount(action gactor.MonitorTimerAction, amount int64) {
	switch action {
	case gactor.MonitorTimerStart:
		m.timerAmount.Add(float64(amount))
	case gactor.MonitorTimerStop:
		m.timerAmount.Sub(float64(amount))
	default:
		m.timerAction.WithLabelValues(action.MonitorKey()).Add(float64(amount))
	}
}

// MonitorRPCAction 上报 RPC 通信事件.
func (m *Metrics) MonitorRPCAction(action gactor.MonitorCAAction) {
	m.commAction.WithLabelValues("rpc", action.MonitorKey()).Inc()
}

// MonitorCastAction 上报 Cast 通信事件.
func (m *Metrics) MonitorCastAction(action gactor.MonitorCAAction) {
	m.commAction.WithLabelValues("cast", action.MonitorKey()).Inc()
}

// MonitorMigrationAction 上报 Actor 迁移事件.
func (m *Metrics) MonitorMigrationAction(action gactor.MonitorMigrationAction) {
	m.migrationAction.WithLabelValues(action.MonitorKey()).Inc()
}

// MonitorMigrationConverged 上报 Actor 迁移收敛耗时.
func (m *Metrics) MonitorMigrationConverged(duration time.Duration) {
	m.migrationConverged.Observe(duration.Seconds())
}
