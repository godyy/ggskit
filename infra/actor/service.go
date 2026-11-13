package actor

import (
	"context"
	"errors"
	"time"

	"github.com/godyy/gactor"
	codecc2s "github.com/godyy/ggskit/base/codec/c2s"
	"github.com/godyy/glog"
	"google.golang.org/protobuf/proto"
)

// ServiceConfig Actor服务配置.
type ServiceConfig struct {
	// Core 核心配置.
	Core *gactor.ServiceConfig

	// Logger 日志记录器.
	Logger glog.Logger

	// S2SProtoReg S2S 协议注册表.
	ProtoRegistry *ProtoRegistry
}

// Service 封装gactor.Service.
type Service struct {
	*ProtoRegistry
	core *gactor.Service
}

// NewService 创建Actor服务.
func NewService(cfg *ServiceConfig) (*Service, error) {
	if cfg == nil {
		return nil, errors.New("service config is nil")
	}
	if cfg.ProtoRegistry == nil {
		return nil, errors.New("proto registry is nil")
	}
	return &Service{
		core:          gactor.NewService(cfg.Core, gactor.WithServiceLogger(cfg.Logger)),
		ProtoRegistry: cfg.ProtoRegistry,
	}, nil
}

// Start 启动Actor服务.
func (s *Service) Start() error {
	return s.core.Start()
}

// Stop 停止Actor服务.
func (s *Service) Stop() error {
	return s.core.Stop()
}

// StartMigration 开始将本节点 Actor 迁移到目标节点。
func (s *Service) StartMigration(targetNodeID string, cb gactor.ServiceMigrationConvergedFunc) error {
	return s.core.StartMigration(targetNodeID, cb)
}

// StartMigrationWithPlanner 开始将本节点 Actor 迁移到目标节点，使用指定的迁移计划器。
func (s *Service) StartMigrationWithPlanner(planner gactor.ServiceMigrationPlanner, cb gactor.ServiceMigrationConvergedFunc) error {
	return s.core.StartMigrationWithPlanner(planner, cb)
}

// MigrationSnapshot 返回当前迁移进度。
func (s *Service) MigrationSnapshot() gactor.ServiceMigrationSnapshot {
	return s.core.MigrationSnapshot()
}

// FinishMigrationForwarding 停止迁移完成后的源端转发。
func (s *Service) FinishMigrationForwarding() error {
	return s.core.FinishMigrationForwarding()
}

// FinalizeMigration 回收源端已迁移 Actor。
func (s *Service) FinalizeMigration(ctx context.Context) error {
	return s.core.FinalizeMigration(ctx)
}

// HandlePacket 处理节点字节数据.
func (s *Service) HandlePacket(remoteNodeId string, data []byte) error {
	return s.core.HandlePacket(remoteNodeId, data)
}

// StartActor 启动Actor.
func (s *Service) StartActor(ctx context.Context, uid ActorUID) error {
	return s.core.StartActor(ctx, uid)
}

// RPCWithDeadline 向 to 指向的 Actor 发起 RPC 调用.
// deadline 指定具体超时时刻.
func (s *Service) RPCWithDeadline(to ActorUID, args proto.Message, deadline time.Time) (proto.Message, error) {
	var (
		argsPayload  S2SPayload
		replyPayload S2SPayload
		err          error
	)

	argsPayload, err = NewS2SPayload(args, s.S2S)
	if err != nil {
		return nil, err
	}

	if err := s.core.RPCWithDeadline(to, &argsPayload, &replyPayload, deadline); err != nil {
		return nil, err
	}

	return replyPayload.Msg, nil
}

// RPCWithTimeout 向 to 指向的 Actor 发起 RPC 调用.
// timeout 指定超时时间.
func (s *Service) RPCWithTimeout(to ActorUID, args proto.Message, timeout time.Duration) (proto.Message, error) {
	return s.RPCWithDeadline(to, args, time.Now().Add(timeout))
}

// RPC 向 to 指向的 Actor 发起 RPC 调用.
// 超时间隔使用配置的默认值.
func (s *Service) RPC(to ActorUID, args proto.Message) (proto.Message, error) {
	var (
		argsPayload  S2SPayload
		replyPayload S2SPayload
		err          error
	)

	argsPayload, err = NewS2SPayload(args, s.S2S)
	if err != nil {
		return nil, err
	}

	if err := s.core.RPC(to, &argsPayload, &replyPayload); err != nil {
		return nil, err
	}

	return replyPayload.Msg, nil
}

// RPCWithContext 向 to 指向的 Actor 发起 RPC 调用.
// 超时 deadline 从 ctx 获取，若未设置, 使用默认超时时间.
// 同时监听上下文, 若上下文取消，提前终止调用.
func (s *Service) RPCWithContext(ctx context.Context, to ActorUID, args proto.Message) (proto.Message, error) {
	var (
		argsPayload  S2SPayload
		replyPayload S2SPayload
		err          error
	)

	argsPayload, err = NewS2SPayload(args, s.S2S)
	if err != nil {
		return nil, err
	}

	if err := s.core.RPCWithContext(ctx, to, &argsPayload, &replyPayload); err != nil {
		return nil, err
	}

	return replyPayload.Msg, nil
}

// ServiceRPCResp Service RPC 响应.
type ServiceRPCResp struct {
	CbCtx any           // 回调上下文.
	Reply proto.Message // 回复消息.
	Err   error         // 错误.
}

// ServiceAsyncRPCFunc Service专用异步RPC回调.
type ServiceAsyncRPCFunc func(resp ServiceRPCResp)

// cbctxServiceAsyncRPC 为 ServiceAsyncRPCFunc 添加上下文.
func cbctxServiceAsyncRPC(cb ServiceAsyncRPCFunc, cbctx any) gactor.CbCtx {
	return gactor.NewCbCtx(cbctxKeyCb, cb, cbctxKeyCbCtx, cbctx)
}

// serviceAsyncRPCCallback Service 异步 RPC 回调.
func serviceAsyncRPCCallback(r gactor.RPCResp) {
	cbctx := r.Ctx().(gactor.CbCtx)
	defer cbctx.Release()
	cb, _ := gactor.CbCtxGet[ServiceAsyncRPCFunc](cbctx, cbctxKeyCb)
	innerCbCtx, _ := gactor.CbCtxGet[any](cbctx, cbctxKeyCbCtx)

	if err := r.Err(); err != nil {
		cb(ServiceRPCResp{CbCtx: innerCbCtx, Reply: nil, Err: err})
		return
	}

	var replyPayload S2SPayload
	if err := r.DecodeReply(&replyPayload); err != nil {
		cb(ServiceRPCResp{CbCtx: innerCbCtx, Reply: nil, Err: err})
		return
	}

	cb(ServiceRPCResp{CbCtx: innerCbCtx, Reply: replyPayload.Msg, Err: nil})
}

// AsyncRPCWithDeadline 向 to 指向的 Actor 发起异步 RPC 调用.
// deadline 指定具体超时时刻.
func (s *Service) AsyncRPCWithDeadline(to ActorUID, args proto.Message, cb ServiceAsyncRPCFunc, cbctx any, deadline time.Time) error {
	argsPayload, err := NewS2SPayload(args, s.S2S)
	if err != nil {
		return err
	}

	cbctx = cbctxServiceAsyncRPC(cb, cbctx)
	if err := s.core.AsyncRPCWithDeadline(to, &argsPayload, serviceAsyncRPCCallback, cbctx, deadline); err != nil {
		cbctx.(gactor.CbCtx).Release()
		return err
	}
	return nil
}

// AsyncRPCWithTimeout 向 to 指向的 Actor 发起异步 RPC 调用.
// timeout 指定超时间隔.
func (s *Service) AsyncRPCWithTimeout(to ActorUID, args proto.Message, cb ServiceAsyncRPCFunc, cbctx any, timeout time.Duration) error {
	return s.AsyncRPCWithDeadline(to, args, cb, cbctx, time.Now().Add(timeout))
}

// AsyncRPC 向 to 指向的 Actor 发起异步 RPC 调用.
func (s *Service) AsyncRPC(to ActorUID, args proto.Message, cb ServiceAsyncRPCFunc, cbctx any) error {
	argsPayload, err := NewS2SPayload(args, s.S2S)
	if err != nil {
		return err
	}

	cbctx = cbctxServiceAsyncRPC(cb, cbctx)
	if err := s.core.AsyncRPC(to, &argsPayload, serviceAsyncRPCCallback, cbctx); err != nil {
		cbctx.(gactor.CbCtx).Release()
		return err
	}
	return nil
}

// AsyncRPCWithContext 向 to 指向的 Actor 发起异步 RPC 调用.
// 超时 deadline 从 ctx 获取，若未设置, 使用默认超时时间.
func (s *Service) AsyncRPCWithContext(ctx context.Context, to ActorUID, args proto.Message, cb ServiceAsyncRPCFunc, cbctx any) error {
	argsPayload, err := NewS2SPayload(args, s.S2S)
	if err != nil {
		return err
	}

	cbctx = cbctxServiceAsyncRPC(cb, cbctx)
	if err := s.core.AsyncRPCWithContext(ctx, to, &argsPayload, serviceAsyncRPCCallback, cbctx); err != nil {
		cbctx.(gactor.CbCtx).Release()
		return err
	}
	return nil
}

// Cast 发送消息到目标actor.
func (s *Service) Cast(to ActorUID, msg proto.Message) error {
	payload, err := NewS2SPayload(msg, s.S2S)
	if err != nil {
		return err
	}
	return s.core.Cast(to, &payload)
}

// Forward 向目标Actor透传消息.
func (s *Service) Forward(to ActorUID, msg proto.Message) error {
	payload, err := NewC2SPayload(codecc2s.PtPush, 0, msg, s.C2S)
	if err != nil {
		return err
	}
	return s.core.Forward(to, &payload)
}
