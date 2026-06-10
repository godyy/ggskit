package pprof

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type serveMuxRouter struct {
	mux *http.ServeMux
}

func (r serveMuxRouter) Handle(method string, path string, handler http.Handler) {
	r.mux.Handle(method+" "+path, handler)
}

type ginRouter struct {
	engine *gin.Engine
}

func (r ginRouter) Handle(method string, path string, handler http.Handler) {
	r.engine.Handle(method, path, gin.WrapH(handler))
}

func TestRegisterHandlerServeMux(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHandler(serveMuxRouter{mux: mux}, "")

	assertResponse(t, mux, http.MethodGet, "/debug/pprof", http.StatusMovedPermanently, "/debug/pprof/")
	assertResponse(t, mux, http.MethodGet, "/debug/pprof/", http.StatusOK, "")
	assertResponse(t, mux, http.MethodGet, "/debug/pprof/cmdline", http.StatusOK, "")
	assertResponse(t, mux, http.MethodGet, "/debug/pprof/symbol", http.StatusOK, "")
	assertResponse(t, mux, http.MethodPost, "/debug/pprof/symbol", http.StatusOK, "")
	assertResponse(t, mux, http.MethodGet, "/debug/pprof/goroutine", http.StatusOK, "")
}

func TestRegisterHandlerGin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterHandler(ginRouter{engine: engine}, "/debug/pprof")

	assertResponse(t, engine, http.MethodGet, "/debug/pprof", http.StatusMovedPermanently, "/debug/pprof/")
	assertResponse(t, engine, http.MethodGet, "/debug/pprof/", http.StatusOK, "")
	assertResponse(t, engine, http.MethodGet, "/debug/pprof/cmdline", http.StatusOK, "")
	assertResponse(t, engine, http.MethodGet, "/debug/pprof/symbol", http.StatusOK, "")
	assertResponse(t, engine, http.MethodPost, "/debug/pprof/symbol", http.StatusOK, "")
	assertResponse(t, engine, http.MethodGet, "/debug/pprof/goroutine", http.StatusOK, "")
}

func TestRegisterHandlerCustomBasePath(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHandler(serveMuxRouter{mux: mux}, "custom/pprof/")

	assertResponse(t, mux, http.MethodGet, "/custom/pprof", http.StatusMovedPermanently, "/custom/pprof/")
	assertResponse(t, mux, http.MethodGet, "/custom/pprof/", http.StatusOK, "")
}

func assertResponse(t *testing.T, handler http.Handler, method string, path string, wantStatus int, wantLocation string) {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d", method, path, resp.Code, wantStatus)
	}
	if got := resp.Header().Get("Location"); got != wantLocation {
		t.Fatalf("%s %s Location = %q, want %q", method, path, got, wantLocation)
	}
}
