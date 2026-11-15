package actor

import "github.com/godyy/gutils/container/kv"

var (
	cbctxKeyCb    = kv.NewK[any]("cb")    // 回调函数.
	cbctxKeyCbCtx = kv.NewK[any]("cbctx") // 回调函数上下文.
)
