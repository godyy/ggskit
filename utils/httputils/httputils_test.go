package httputils

import (
	"testing"
)

func TestPostJson(t *testing.T) {
	resp := &map[string]string{}
	if err := PostJson("http://localhost:8080", map[string]string{"name": "godyy"}, resp, nil); err != nil {
		t.Fatalf("post json failed: %v", err)
	}
}
