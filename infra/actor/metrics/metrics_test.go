package metrics

import (
	"testing"
	"time"

	"github.com/godyy/gactor"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func newTestMetrics(t *testing.T) *Metrics {
	t.Helper()
	m, err := New(&Config{
		Registry:        prom.NewRegistry(),
		Namespace:       "test",
		CategoryLabeler: func(category gactor.ActorCategory) string { return "c" + string(rune('0'+category)) },
	})
	if err != nil {
		t.Fatalf("new metrics: %v", err)
	}
	return m
}

func TestMonitorActorAmount(t *testing.T) {
	m := newTestMetrics(t)
	category := gactor.ActorCategory(1)

	m.MonitorActorAmount(gactor.MonitorActorStart, category, 1)
	m.MonitorActorAmount(gactor.MonitorActorStart, category, 1)
	if got := testutil.ToFloat64(m.actorAmount.WithLabelValues("c1")); got != 2 {
		t.Fatalf("actor amount want 2, got %v", got)
	}

	m.MonitorActorAmount(gactor.MonitorActorStop, category, 1)
	if got := testutil.ToFloat64(m.actorAmount.WithLabelValues("c1")); got != 1 {
		t.Fatalf("actor amount want 1, got %v", got)
	}

	m.MonitorActorAmount(gactor.MonitorActorPanic, category, 1)
	if got := testutil.ToFloat64(m.actorAction.WithLabelValues("c1", gactor.MonitorActorPanic.MonitorKey())); got != 1 {
		t.Fatalf("actor panic want 1, got %v", got)
	}
}

func TestMonitorTimerAmount(t *testing.T) {
	m := newTestMetrics(t)

	m.MonitorTimerAmount(gactor.MonitorTimerStart, 3)
	if got := testutil.ToFloat64(m.timerAmount); got != 3 {
		t.Fatalf("timer amount want 3, got %v", got)
	}

	m.MonitorTimerAmount(gactor.MonitorTimerStop, 1)
	if got := testutil.ToFloat64(m.timerAmount); got != 2 {
		t.Fatalf("timer amount want 2, got %v", got)
	}

	m.MonitorTimerAmount(gactor.MonitorTimerTrigger, 5)
	if got := testutil.ToFloat64(m.timerAction.WithLabelValues(gactor.MonitorTimerTrigger.MonitorKey())); got != 5 {
		t.Fatalf("timer trigger want 5, got %v", got)
	}
}

func TestMonitorCommAction(t *testing.T) {
	m := newTestMetrics(t)

	m.MonitorRPCAction(gactor.MonitorCARPC)
	m.MonitorRPCAction(gactor.MonitorCARPCTimeout)
	m.MonitorCastAction(gactor.MonitorCACast)

	if got := testutil.ToFloat64(m.commAction.WithLabelValues("rpc", gactor.MonitorCARPC.MonitorKey())); got != 1 {
		t.Fatalf("rpc want 1, got %v", got)
	}
	if got := testutil.ToFloat64(m.commAction.WithLabelValues("rpc", gactor.MonitorCARPCTimeout.MonitorKey())); got != 1 {
		t.Fatalf("rpc timeout want 1, got %v", got)
	}
	if got := testutil.ToFloat64(m.commAction.WithLabelValues("cast", gactor.MonitorCACast.MonitorKey())); got != 1 {
		t.Fatalf("cast want 1, got %v", got)
	}
}

func TestMonitorMigration(t *testing.T) {
	m := newTestMetrics(t)

	m.MonitorMigrationAction(gactor.MonitorMigrationTakeOverSuccess)
	if got := testutil.ToFloat64(m.migrationAction.WithLabelValues(gactor.MonitorMigrationTakeOverSuccess.MonitorKey())); got != 1 {
		t.Fatalf("migration takeover success want 1, got %v", got)
	}

	m.MonitorMigrationConverged(1500 * time.Millisecond)
	if got := testutil.CollectAndCount(m.migrationConverged); got != 1 {
		t.Fatalf("migration converged metric count want 1, got %v", got)
	}
}

func TestNewDefaultRegistry(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("new metrics with nil config: %v", err)
	}
	if m.Registry() == nil {
		t.Fatal("registry should not be nil")
	}
	// 默认分类标签使用十进制字符串.
	m.MonitorActorAmount(gactor.MonitorActorStart, gactor.ActorCategory(2), 1)
	if got := testutil.ToFloat64(m.actorAmount.WithLabelValues("2")); got != 1 {
		t.Fatalf("actor amount want 1, got %v", got)
	}
}
