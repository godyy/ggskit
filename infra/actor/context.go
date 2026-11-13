package actor

import (
	"context"
	"time"

	"github.com/godyy/gactor"
	codecc2s "github.com/godyy/ggskit/base/codec/c2s"
	"github.com/godyy/ggskit/base/protocol"
	"google.golang.org/protobuf/proto"
)

// Context
type Context = gactor.Context

// ContextSuspender
type ContextSuspender = gactor.ContextSuspender

// ContextFuncArgs
type ContextFuncArgs = gactor.ContextFuncArgs

// ContextFunc
type ContextFunc = gactor.ContextFunc

type ctxK struct{}

// CtxK 上下文kv的key.
type CtxK[V any] struct {
	*ctxK
}

// CtxV 上下文kv的value.
type CtxV = any

func NewCtxK[V CtxV]() CtxK[V] {
	return CtxK[V]{ctxK: &ctxK{}}
}

// CtxKSet 通过CtxK设置Value.
func CtxKSet[V CtxV](ctx *Context, k CtxK[V], v V) {
	ctx.Set(k, v)
}

// CtxKGet 通过CtxK获取Value.
func CtxKGet[V CtxV](ctx *Context, k CtxK[V]) (v V, exists bool) {
	var vv any
	vv, exists = ctx.Get(k)
	if exists {
		v = vv.(V)
	}
	return
}

// CtxActor 获取上下文中的 Actor, 泛型支持.
func CtxActor[Actor ActorBehavior](ctx *Context) Actor {
	return ctx.Actor().Behavior().(Actor)
}

// CtxActorModule 获取上下文中的 Actor 模块, 泛型支持.
func CtxActorModule[M Module](ctx *Context, autoCreate bool) M {
	return GetActorModule[M](ctx.Actor().Behavior().(ActorWithModule), autoCreate)
}

// ctxKSeq	 用于存储请求序号的key.
var ctxKSeq = NewCtxK[uint32]()

// ContextSugarUtil 上下文语法糖工具.
type ContextSugarUtil struct {
	*ProtoRegistry
}

// NewContextSugarUtil 创建上下文语法糖工具.
func NewContextSugarUtil(protoRegistry *ProtoRegistry) *ContextSugarUtil {
	return &ContextSugarUtil{
		ProtoRegistry: protoRegistry,
	}
}

// Decode 解码请求消息.
func (h *ContextSugarUtil) Decode(ctx *Context) (protocol.PID, proto.Message, error) {
	if ctx.RequestType() == gactor.RequestTypeReq {
		var payload C2SPayload
		if err := ctx.Decode(&payload); err != nil {
			return 0, nil, err
		}
		CtxKSet(ctx, ctxKSeq, payload.Seq)
		return payload.PID, payload.Msg, nil
	} else {
		var payload S2SPayload
		if err := ctx.Decode(&payload); err != nil {
			return 0, nil, err
		}
		return payload.PID, payload.Msg, nil
	}
}

// Reply 回复.
func (h *ContextSugarUtil) Reply(ctx *Context, reply proto.Message) error {
	if ctx.RequestType() == gactor.RequestTypeReq {
		seq, ok := CtxKGet(ctx, ctxKSeq)
		if !ok {
			panic("ctx has no seq")
		}
		payload, err := NewC2SPayload(codecc2s.PtResp, seq, reply, h.C2S)
		if err != nil {
			return err
		}
		return ctx.Reply(&payload)
	} else {
		payload, err := NewS2SPayload(reply, h.S2S)
		if err != nil {
			return err
		}
		return ctx.Reply(&payload)
	}
}

// RPCWithDeadline
func (h *ContextSugarUtil) RPCWithDeadline(ctx *Context, to ActorUID, args proto.Message, deadline time.Time) (proto.Message, error) {
	var (
		argsPayload  S2SPayload
		replyPayload S2SPayload
		err          error
	)

	argsPayload, err = NewS2SPayload(args, h.S2S)
	if err != nil {
		return nil, err
	}

	err = ctx.RPCWithDeadline(to, &argsPayload, &replyPayload, deadline)
	if err != nil {
		return nil, err
	}

	return replyPayload.Msg, nil
}

// RPCWithTimeout
func (h *ContextSugarUtil) RPCWithTimeout(ctx *Context, to ActorUID, args proto.Message, timeout time.Duration) (proto.Message, error) {
	return h.RPCWithDeadline(ctx, to, args, time.Now().Add(timeout))
}

// RPC
func (h *ContextSugarUtil) RPC(ctx *Context, to ActorUID, args proto.Message) (proto.Message, error) {
	var (
		argsPayload  S2SPayload
		replyPayload S2SPayload
		err          error
	)

	argsPayload, err = NewS2SPayload(args, h.S2S)
	if err != nil {
		return nil, err
	}

	err = ctx.RPC(to, &argsPayload, &replyPayload)
	if err != nil {
		return nil, err
	}

	return replyPayload.Msg, nil
}

// RPCWithContext
func (h *ContextSugarUtil) RPCWithContext(ctx *Context, cctx context.Context, to ActorUID, args proto.Message) (proto.Message, error) {
	var (
		argsPayload  S2SPayload
		replyPayload S2SPayload
		err          error
	)

	argsPayload, err = NewS2SPayload(args, h.S2S)
	if err != nil {
		return nil, err
	}

	err = ctx.RPCWithContext(cctx, to, &argsPayload, &replyPayload)
	if err != nil {
		return nil, err
	}

	return replyPayload.Msg, nil
}

// ContextRPCResp 上下文 RPC 响应.
type ContextRPCResp struct {
	Ctx   *Context      // 上下文.
	CbCtx any           // 回调上下文.
	Reply proto.Message // 回复消息.
	Err   error         // 错误.
}

// ContextAsyncRPCFunc 上下文异步 RPC 回调.
type ContextAsyncRPCFunc func(resp ContextRPCResp)

// cbctxContextAsyncRPC 上下文异步 RPC 回调上下文.
func cbctxContextAsyncRPC(cb ContextAsyncRPCFunc, cbctx any) gactor.CbCtx {
	return gactor.NewCbCtx(cbctxKeyCb, cb, cbctxKeyCbCtx, cbctx)
}

// contextAsyncRPCCallback 上下文异步 RPC 回调.
func contextAsyncRPCCallback(ctx *Context, resp gactor.RPCResp) {
	cbctx := resp.Ctx().(gactor.CbCtx)
	defer cbctx.Release()
	cb, _ := gactor.CbCtxGet[ContextAsyncRPCFunc](cbctx, cbctxKeyCb)
	innerCbCtx, _ := gactor.CbCtxGet[any](cbctx, cbctxKeyCbCtx)

	if err := resp.Err(); err != nil {
		cb(ContextRPCResp{Ctx: ctx, CbCtx: innerCbCtx, Reply: nil, Err: err})
		return
	}

	var replyPayload S2SPayload
	if err := resp.DecodeReply(&replyPayload); err != nil {
		cb(ContextRPCResp{Ctx: ctx, CbCtx: innerCbCtx, Reply: nil, Err: err})
		return
	}

	cb(ContextRPCResp{Ctx: ctx, CbCtx: innerCbCtx, Reply: replyPayload.Msg, Err: nil})
}

// AsyncRPCWithDeadline
func (h *ContextSugarUtil) AsyncRPCWithDeadline(ctx *Context, to ActorUID, args proto.Message, cb ContextAsyncRPCFunc, cbctx any, deadline time.Time) error {
	argsPayload, err := NewS2SPayload(args, h.S2S)
	if err != nil {
		return err
	}

	cbctx = cbctxContextAsyncRPC(cb, cbctx)
	if err := ctx.AsyncRPCWithDeadline(to, &argsPayload, contextAsyncRPCCallback, cbctx, deadline); err != nil {
		cbctx.(gactor.CbCtx).Release()
		return err
	}
	return nil
}

// AsyncRPCWithTimeout
func (h *ContextSugarUtil) AsyncRPCWithTimeout(ctx *Context, to ActorUID, args proto.Message, cb ContextAsyncRPCFunc, cbctx any, timeout time.Duration) error {
	return h.AsyncRPCWithDeadline(ctx, to, args, cb, cbctx, time.Now().Add(timeout))
}

// AsyncRPC
func (h *ContextSugarUtil) AsyncRPC(ctx *Context, to ActorUID, args proto.Message, cb ContextAsyncRPCFunc, cbctx any) error {
	argsPayload, err := NewS2SPayload(args, h.S2S)
	if err != nil {
		return err
	}

	cbctx = cbctxContextAsyncRPC(cb, cbctx)
	if err := ctx.AsyncRPC(to, &argsPayload, contextAsyncRPCCallback, cbctx); err != nil {
		cbctx.(gactor.CbCtx).Release()
		return err
	}
	return nil
}

// AsyncRPCWithContext
func (h *ContextSugarUtil) AsyncRPCWithContext(ctx *Context, cctx context.Context, to ActorUID, args proto.Message, cb ContextAsyncRPCFunc, cbctx any) error {
	argsPayload, err := NewS2SPayload(args, h.S2S)
	if err != nil {
		return err
	}

	cbctx = cbctxContextAsyncRPC(cb, cbctx)
	if err := ctx.AsyncRPCWithContext(cctx, to, &argsPayload, contextAsyncRPCCallback, cbctx); err != nil {
		cbctx.(gactor.CbCtx).Release()
		return err
	}
	return nil
}

// Cast
func (h *ContextSugarUtil) Cast(ctx *Context, to ActorUID, msg proto.Message) error {
	payload, err := NewS2SPayload(msg, h.S2S)
	if err != nil {
		return err
	}
	return ctx.Cast(to, payload)
}
