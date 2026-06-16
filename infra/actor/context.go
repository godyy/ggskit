package actor

import (
	"context"
	"time"

	"github.com/godyy/gactor"
	codecc2s "github.com/godyy/ggskit/base/codec/c2s"
	"google.golang.org/protobuf/proto"
)

const (
	ctxKeySeq = "ctx:seq"
)

// Context
type Context = gactor.Context

// CtxActor 获取上下文中的 Actor, 泛型支持.
func CtxActor[Actor ActorBehavior](ctx *Context) Actor {
	return ctx.Actor().Behavior().(Actor)
}

// ctxSeq 获取上下中的请求序号, ctxKeySeq.
func ctxSeq(ctx *Context) (uint32, bool) {
	if v, ok := ctx.Get(ctxKeySeq); ok {
		return v.(uint32), true
	} else {
		return 0, false
	}
}

// ContextHelper 上下文帮助类.
type ContextHelper struct {
	*ProtoRegistry
}

// NewContextHelper 创建上下文帮助类.
func NewContextHelper(protoRegistry *ProtoRegistry) *ContextHelper {
	return &ContextHelper{
		ProtoRegistry: protoRegistry,
	}
}

// Decode 解码请求消息.
func (h *ContextHelper) Decode(ctx *Context) (uint16, proto.Message, error) {
	if ctx.RequestType() == gactor.RequestTypeReq {
		var payload C2SPayload
		if err := ctx.Decode(&payload); err != nil {
			return 0, nil, err
		}
		ctx.Set(ctxKeySeq, payload.Seq)
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
func (h *ContextHelper) Reply(ctx *Context, reply proto.Message) error {
	if ctx.RequestType() == gactor.RequestTypeReq {
		seq, ok := ctxSeq(ctx)
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
func (h *ContextHelper) RPCWithDeadline(ctx *Context, to ActorUID, args proto.Message, deadline time.Time) (proto.Message, error) {
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
func (h *ContextHelper) RPCWithTimeout(ctx *Context, to ActorUID, args proto.Message, timeout time.Duration) (proto.Message, error) {
	return h.RPCWithDeadline(ctx, to, args, time.Now().Add(timeout))
}

// RPC
func (h *ContextHelper) RPC(ctx *Context, to ActorUID, args proto.Message) (proto.Message, error) {
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
func (h *ContextHelper) RPCWithContext(ctx *Context, cctx context.Context, to ActorUID, args proto.Message) (proto.Message, error) {
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

// ContextAsyncRPCCallback 上下文异步 RPC 回调.
type ContextAsyncRPCCallback func(ctx *Context, reply proto.Message, err error)

// ContextAsyncRPCCallback 上下文异步 RPC 回调.
func (h *ContextHelper) handleAsyncRPCResp(ctx *Context, resp *gactor.RPCResp, callback ContextAsyncRPCCallback) {
	if err := resp.Err(); err != nil {
		callback(ctx, nil, err)
		return
	}

	var replyPayload S2SPayload
	if err := resp.DecodeReply(&replyPayload); err != nil {
		callback(ctx, nil, err)
		return
	}

	callback(ctx, replyPayload.Msg, nil)
}

// AsyncRPCWithDeadline
func (h *ContextHelper) AsyncRPCWithDeadline(ctx *Context, to ActorUID, args proto.Message, callback ContextAsyncRPCCallback, deadline time.Time) error {
	argsPayload, err := NewS2SPayload(args, h.S2S)
	if err != nil {
		return err
	}

	return ctx.AsyncRPCWithDeadline(to, &argsPayload, func(ctx *Context, resp *gactor.RPCResp) {
		h.handleAsyncRPCResp(ctx, resp, callback)
	}, deadline)
}

// AsyncRPCWithTimeout
func (h *ContextHelper) AsyncRPCWithTimeout(ctx *Context, to ActorUID, args proto.Message, callback ContextAsyncRPCCallback, timeout time.Duration) error {
	return h.AsyncRPCWithDeadline(ctx, to, args, callback, time.Now().Add(timeout))
}

// AsyncRPC
func (h *ContextHelper) AsyncRPC(ctx *Context, to ActorUID, args proto.Message, callback ContextAsyncRPCCallback) error {
	argsPayload, err := NewS2SPayload(args, h.S2S)
	if err != nil {
		return err
	}

	return ctx.AsyncRPC(to, &argsPayload, func(ctx *Context, resp *gactor.RPCResp) {
		h.handleAsyncRPCResp(ctx, resp, callback)
	})
}

// AsyncRPCWithContext
func (h *ContextHelper) AsyncRPCWithContext(ctx *Context, cctx context.Context, to ActorUID, args proto.Message, callback ContextAsyncRPCCallback) error {
	argsPayload, err := NewS2SPayload(args, h.S2S)
	if err != nil {
		return err
	}

	return ctx.AsyncRPCWithContext(cctx, to, &argsPayload, func(ctx *Context, resp *gactor.RPCResp) {
		h.handleAsyncRPCResp(ctx, resp, callback)
	})
}

// Cast
func (h *ContextHelper) Cast(ctx *Context, to ActorUID, msg proto.Message) error {
	payload, err := NewS2SPayload(msg, h.S2S)
	if err != nil {
		return err
	}
	return ctx.Cast(to, payload)
}
