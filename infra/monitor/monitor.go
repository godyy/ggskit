package monitor

import "net/http"

type HttpRouter interface {
	Handle(method string, path string, handler http.Handler)
}
