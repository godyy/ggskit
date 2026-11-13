package actor

import (
	"context"
	"time"

	"github.com/godyy/gactor"
	codecc2s "github.com/godyy/ggskit/base/codec/c2s"
	"google.golang.org/protobuf/proto"
)

// Actor
type Actor = gactor.Actor

// CActor
type CActor = gactor.CActor

// ActorBehavior
type ActorBehavior interface {
	gactor.ActorBehavior

	// GetActor 获取Actor实例.
	GetActor() Actor
}

// CActorBehavior
type CActorBehavior interface {
	ActorBehavior

	// GetCActor 获取CActor实例.
	GetCActor() CActor
}

// ActorID.
type ActorID = gactor.ActorID

// ActorUID
type ActorUID = gactor.ActorUID

// ToActor 将 Actor 转换为指定行为的 Actor.
func ToActor[AB ActorBehavior](actor Actor) AB {
	return actor.Behavior().(AB)
}

// ActorSugarUtil Actor语法糖工具.
type ActorSugarUtil struct {
	*ProtoRegistry
}

// NewActorSugarUtil 创建Actor语法糖工具.
func NewActorSugarUtil(protoReg *ProtoRegistry) *ActorSugarUtil {
	return &ActorSugarUtil{
		ProtoRegistry: protoReg,
	}
}

// PushRawMessage 向客户端推送消息.
func (h *ActorSugarUtil) PushRawMessage(actor CActor, msg proto.Message) error {
	payload, err := NewC2SPayload(codecc2s.PtPush, 0, msg, h.C2S)
	if err != nil {
		return err
	}
	return actor.PushRawMessage(&payload)
}

// RPCWithDeadline 向 to 指向的 Actor 发起 RPC 调用.
// deadline 为超时时间.
func (h *ActorSugarUtil) RPCWithDeadline(actor Actor, to ActorUID, args proto.Message, deadline time.Time) (proto.Message, error) {
	var (
		argsPayload  S2SPayload
		replyPayload S2SPayload
		err          error
	)

	argsPayload, err = NewS2SPayload(args, h.S2S)
	if err != nil {
		return nil, err
	}

	if err := actor.RPCWithDeadline(to, &argsPayload, &replyPayload, deadline); err != nil {
		return nil, err
	}

	return replyPayload.Msg, nil
}

// RPCWithTimeout 向 to 指向的 Actor 发起 RPC 调用.
// timeout 为超时间隔.
func (h *ActorSugarUtil) RPCWithTimeout(actor Actor, to ActorUID, args proto.Message, timeout time.Duration) (proto.Message, error) {
	return h.RPCWithDeadline(actor, to, args, time.Now().Add(timeout))
}

// RPC 向 to 指向的 Actor 发起 RPC 调用.
// 使用配置的默认超时间隔.
func (h *ActorSugarUtil) RPC(actor Actor, to ActorUID, args proto.Message) (proto.Message, error) {
	var (
		argsPayload  S2SPayload
		replyPayload S2SPayload
		err          error
	)

	argsPayload, err = NewS2SPayload(args, h.S2S)
	if err != nil {
		return nil, err
	}

	if err := actor.RPC(to, &argsPayload, &replyPayload); err != nil {
		return nil, err
	}

	return replyPayload.Msg, nil
}

// RPCWithContext 向 to 指向的 Actor 发起 RPC 调用.
// 超时 deadline 从 ctx 获取，若未设置, 使用默认超时时间.
func (h *ActorSugarUtil) RPCWithContext(ctx context.Context, actor Actor, to ActorUID, args proto.Message) (proto.Message, error) {
	var (
		argsPayload  S2SPayload
		replyPayload S2SPayload
		err          error
	)

	argsPayload, err = NewS2SPayload(args, h.S2S)
	if err != nil {
		return nil, err
	}

	if err := actor.RPCWithContext(ctx, to, &argsPayload, &replyPayload); err != nil {
		return nil, err
	}

	return replyPayload.Msg, nil
}

// ActorRPCResp Actor RPC 响应.
type ActorRPCResp struct {
	Actor Actor         // Actor.
	CbCtx any           // 回调上下文.
	Reply proto.Message // 回复消息.
	Err   error         // 错误.
}

// ActorAsyncRPCFunc Actor异步RPC回调.
type ActorAsyncRPCFunc func(resp ActorRPCResp)

// cbctxActorAsyncRPC 创建异步RPC回调上下文.
func cbctxActorAsyncRPC(cb ActorAsyncRPCFunc, cbctx any) gactor.CbCtx {
	return gactor.NewCbCtx(cbctxKeyCb, cb, cbctxKeyCbCtx, cbctx)
}

// actorAsyncRPCCallback Actor异步RPC回调.
func actorAsyncRPCCallback(actor Actor, resp gactor.RPCResp) {
	cbctx := resp.Ctx().(gactor.CbCtx)
	defer cbctx.Release()
	cb, _ := gactor.CbCtxGet[ActorAsyncRPCFunc](cbctx, cbctxKeyCb)
	innerCbCtx, _ := gactor.CbCtxGet[any](cbctx, cbctxKeyCbCtx)

	if err := resp.Err(); err != nil {
		cb(ActorRPCResp{Actor: actor, CbCtx: innerCbCtx, Reply: nil, Err: err})
		return
	}

	var replyPayload S2SPayload
	if err := resp.DecodeReply(&replyPayload); err != nil {
		cb(ActorRPCResp{Actor: actor, CbCtx: innerCbCtx, Reply: nil, Err: err})
		return
	}

	cb(ActorRPCResp{Actor: actor, CbCtx: innerCbCtx, Reply: replyPayload.Msg, Err: nil})
}

// AsyncRPCWithDeadline 向 to 指向的 Actor 发起异步 RPC 调用.
// deadline 为超时时间.
func (h *ActorSugarUtil) AsyncRPCWithDeadline(actor Actor, to ActorUID, args proto.Message, cb ActorAsyncRPCFunc, cbctx any, deadline time.Time) error {
	argsPayload, err := NewS2SPayload(args, h.S2S)
	if err != nil {
		return err
	}

	cbctx = cbctxActorAsyncRPC(cb, cbctx)
	if err := actor.AsyncRPCWithDeadline(to, &argsPayload, actorAsyncRPCCallback, cbctx, deadline); err != nil {
		cbctx.(gactor.CbCtx).Release()
		return err
	}
	return nil
}

// AsyncRPCWithTimeout 向 to 指向的 Actor 发起异步 RPC 调用.
// timeout 为超时间隔.
func (h *ActorSugarUtil) AsyncRPCWithTimeout(actor Actor, to ActorUID, args proto.Message, cb ActorAsyncRPCFunc, cbctx any, timeout time.Duration) error {
	return h.AsyncRPCWithDeadline(actor, to, args, cb, cbctx, time.Now().Add(timeout))
}

// AsyncRPC 向 to 指向的 Actor 发起异步 RPC 调用.
// 使用配置的默认超时间隔.
func (h *ActorSugarUtil) AsyncRPC(actor Actor, to ActorUID, args proto.Message, cb ActorAsyncRPCFunc, cbctx any) error {
	argsPayload, err := NewS2SPayload(args, h.S2S)
	if err != nil {
		return err
	}

	cbctx = cbctxActorAsyncRPC(cb, cbctx)
	if err := actor.AsyncRPC(to, &argsPayload, actorAsyncRPCCallback, cbctx); err != nil {
		cbctx.(gactor.CbCtx).Release()
		return err
	}
	return nil
}

// AsyncRPCWithContext 向 to 指向的 Actor 发起异步 RPC 调用.
// 超时 deadline 从 ctx 获取，若未设置, 使用默认超时时间.
func (h *ActorSugarUtil) AsyncRPCWithContext(ctx context.Context, actor Actor, to ActorUID, args proto.Message, cb ActorAsyncRPCFunc, cbctx any) error {
	argsPayload, err := NewS2SPayload(args, h.S2S)
	if err != nil {
		return err
	}
	cbctx = cbctxActorAsyncRPC(cb, cbctx)
	if err := actor.AsyncRPCWithContext(ctx, to, &argsPayload, actorAsyncRPCCallback, cbctx); err != nil {
		cbctx.(gactor.CbCtx).Release()
		return err
	}
	return nil
}

// Cast 向 to 指向的 Actor 投递消息.
// payload 为投递的负载消息.
func (h *ActorSugarUtil) Cast(actor Actor, to ActorUID, payload proto.Message) error {
	s2sPayload, err := NewS2SPayload(payload, h.S2S)
	if err != nil {
		return err
	}

	return actor.Cast(to, &s2sPayload)
}
