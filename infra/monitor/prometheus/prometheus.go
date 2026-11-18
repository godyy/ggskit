// Package prometheus 提供服务层面的 Prometheus 指标暴露入口.
//
// 指标采集由 infra/actor/metrics 等具体模块负责, 本包只负责注册表构建与 HTTP 暴露.
package prometheus

import (
	"net/http"
	"strings"

	"github.com/godyy/ggskit/infra/monitor"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRegistry 创建注册表, 并预注册 Go 运行时与进程指标采集器.
func NewRegistry() *prom.Registry {
	registry := prom.NewRegistry()
	registry.MustRegister(
		prom.NewGoCollector(),
		prom.NewProcessCollector(prom.ProcessCollectorOpts{}),
	)
	return registry
}

// RegisterHandler 注册 Prometheus 指标路由, 默认路径为 /metrics.
// gatherer 通常为某个注册表, 例如 NewRegistry 或 metrics.Metrics.Registry 的返回值.
func RegisterHandler(router monitor.HttpRouter, gatherer prom.Gatherer, basePath string) {
	base := normalizeBase(basePath)
	router.Handle(http.MethodGet, base, promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
}

// normalizeBase 标准化路径, 确保以 / 开头, 不以 / 结尾.
func normalizeBase(p string) string {
	b := strings.TrimSpace(p)
	if b == "" {
		b = "/metrics"
	}
	b = strings.TrimSuffix(b, "/")
	if !strings.HasPrefix(b, "/") {
		b = "/" + b
	}
	return b
}
