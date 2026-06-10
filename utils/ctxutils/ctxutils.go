package ctxutils

import (
	"context"
	"time"

	"github.com/godyy/ggskit/base/env"
)

// WithTimeout 封装 context.WithTimeout，提供统一的调试模式支持.
func WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if env.Get().Debug() {
		timeout = time.Hour * 1
	}
	return context.WithTimeout(ctx, timeout)
}
