package pprof

import (
	"net/http"
	stdpprof "net/http/pprof"
	"strings"

	"github.com/godyy/ggskit/infra/monitor"
)

// RegisterHandler 注册 pprof 路由, 默认路径为 /debug/pprof。
func RegisterHandler(router monitor.HttpRouter, basePath string) {
	base := normalizeBase(basePath)

	router.Handle(http.MethodGet, base, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, base+"/", http.StatusMovedPermanently)
	}))

	router.Handle(http.MethodGet, base+"/", http.HandlerFunc(stdpprof.Index))
	router.Handle(http.MethodGet, base+"/cmdline", http.HandlerFunc(stdpprof.Cmdline))
	router.Handle(http.MethodGet, base+"/profile", http.HandlerFunc(stdpprof.Profile))
	router.Handle(http.MethodGet, base+"/symbol", http.HandlerFunc(stdpprof.Symbol))
	router.Handle(http.MethodPost, base+"/symbol", http.HandlerFunc(stdpprof.Symbol))
	router.Handle(http.MethodGet, base+"/trace", http.HandlerFunc(stdpprof.Trace))

	router.Handle(http.MethodGet, base+"/goroutine", stdpprof.Handler("goroutine"))
	router.Handle(http.MethodGet, base+"/heap", stdpprof.Handler("heap"))
	router.Handle(http.MethodGet, base+"/allocs", stdpprof.Handler("allocs"))
	router.Handle(http.MethodGet, base+"/block", stdpprof.Handler("block"))
	router.Handle(http.MethodGet, base+"/mutex", stdpprof.Handler("mutex"))
	router.Handle(http.MethodGet, base+"/threadcreate", stdpprof.Handler("threadcreate"))
}

// normalizeBase 标准化路径，确保以 / 开头，不以 / 结尾。
func normalizeBase(p string) string {
	b := strings.TrimSpace(p)
	if b == "" {
		b = "/debug/pprof"
	}
	b = strings.TrimSuffix(b, "/")
	if !strings.HasPrefix(b, "/") {
		b = "/" + b
	}
	return b
}
