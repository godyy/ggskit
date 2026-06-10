package httputils

import "net/http"

type RequestOption func(*http.Request)

func WithHeadSet(key, value string) RequestOption {
	return func(r *http.Request) {
		r.Header.Set(key, value)
	}
}

func WithHeadAdd(key, value string) RequestOption {
	return func(r *http.Request) {
		r.Header.Add(key, value)
	}
}

func WithHeaderFunc(f func(http.Header)) RequestOption {
	return func(r *http.Request) {
		f(r.Header)
	}
}
