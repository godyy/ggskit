package mongobd

import (
	"context"
	"fmt"
	"reflect"

	"github.com/godyy/ggskit/utils/ctxutils"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
)

// worker 后台worker, 负责处理排好队的操作符.
type worker struct {
	*BD         // 归属后台
	ops chan Op // 操作符队列
}

// run woker运行函数.
func (w *worker) run() {
	defer w.BD.wg.Done()
	for op := range w.ops {
		w.safeExec(w.BD.client, op)
	}
}

// stop 停止worker.
func (w *worker) stop() {
	close(w.ops)
}

// safeExec 安全执行操作符.
// 如果操作符执行过程中发生panic, 则会捕获并记录日志, 并将操作符标记为失败.
func (w *worker) safeExec(client *mongo.Client, op Op) {
	ctx := op.base().ctx
	if err := ctx.Err(); err != nil {
		opDone(op, err)
		return
	}

	defer func() {
		if err := recover(); err != nil {
			w.BD.logger.ErrorFields("exec op panic",
				zap.Dict("op",
					zap.String("type", reflect.TypeOf(op).String()),
					zap.Any("value", op),
				),
				zap.Any("error", err),
				zap.StackSkip("stack", 1),
			)
			opDone(op, fmt.Errorf("panic: %v", err))
		}
	}()

	bakCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = ctxutils.WithTimeout(ctx, w.BD.defExecTimeout)
		defer cancel()
		op.base().ctx = ctx
	}

	op.exec(client)
	op.base().ctx = bakCtx
}
