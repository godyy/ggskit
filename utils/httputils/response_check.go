package httputils

import (
	"fmt"
	"net/http"
	"strings"
)

type ResponseChecker func(resp *http.Response) error

func WithCheckStatus(statusCode int) ResponseChecker {
	return func(resp *http.Response) error {
		if resp.StatusCode != statusCode {
			return fmt.Errorf("status code %d not %d", resp.StatusCode, statusCode)
		}
		return nil
	}
}

func WithCheckHead(key, value string, contain bool) ResponseChecker {
	return func(resp *http.Response) error {
		if contain {
			if !strings.Contains(resp.Header.Get(key), value) {
				return fmt.Errorf("header(%s) \"%s\" not contain \"%s\"", key, resp.Header.Get(key), value)
			}
		} else {
			if resp.Header.Get(key) != value {
				return fmt.Errorf("header(%s) \"%s\" not \"%s\"", key, resp.Header.Get(key), value)
			}
		}
		return nil
	}
}

func WithCheckHeader(f func(header http.Header) error) ResponseChecker {
	return func(resp *http.Response) error {
		return f(resp.Header)
	}
}
