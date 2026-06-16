package actor

import (
	"context"
	"errors"
	"time"

	"github.com/godyy/gactor"
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

// ServiceAsyncRPCCallback Service专用异步RPC回调.
type ServiceAsyncRPCCallback func(reply proto.Message, err error)

func (s *Service) handleAsyncRPCResp(r *gactor.RPCResp, callback ServiceAsyncRPCCallback) {
	if err := r.Err(); err != nil {
		callback(nil, err)
		return
	}

	var replyPayload S2SPayload
	if err := r.DecodeReply(&replyPayload); err != nil {
		callback(nil, err)
		return
	}

	callback(replyPayload.Msg, nil)
}

// AsyncRPCWithDeadline 向 to 指向的 Actor 发起异步 RPC 调用.
// deadline 指定具体超时时刻.
func (s *Service) AsyncRPCWithDeadline(to ActorUID, args proto.Message, callback ServiceAsyncRPCCallback, deadline time.Time) error {
	argsPayload, err := NewS2SPayload(args, s.S2S)
	if err != nil {
		return err
	}

	return s.core.AsyncRPCWithDeadline(to, &argsPayload, func(r *gactor.RPCResp) {
		s.handleAsyncRPCResp(r, callback)
	}, deadline)
}

// AsyncRPCWithTimeout 向 to 指向的 Actor 发起异步 RPC 调用.
// timeout 指定超时间隔.
func (s *Service) AsyncRPCWithTimeout(to ActorUID, args proto.Message, callback ServiceAsyncRPCCallback, timeout time.Duration) error {
	return s.AsyncRPCWithDeadline(to, args, callback, time.Now().Add(timeout))
}

// AsyncRPC 向 to 指向的 Actor 发起异步 RPC 调用.
func (s *Service) AsyncRPC(to ActorUID, args proto.Message, callback ServiceAsyncRPCCallback) error {
	argsPayload, err := NewS2SPayload(args, s.S2S)
	if err != nil {
		return err
	}

	return s.core.AsyncRPC(to, &argsPayload, func(r *gactor.RPCResp) {
		s.handleAsyncRPCResp(r, callback)
	})
}

// AsyncRPCWithContext 向 to 指向的 Actor 发起异步 RPC 调用.
// 超时 deadline 从 ctx 获取，若未设置, 使用默认超时时间.
func (s *Service) AsyncRPCWithContext(ctx context.Context, to ActorUID, args proto.Message, callback ServiceAsyncRPCCallback) error {
	argsPayload, err := NewS2SPayload(args, s.S2S)
	if err != nil {
		return err
	}

	return s.core.AsyncRPCWithContext(ctx, to, &argsPayload, func(r *gactor.RPCResp) {
		s.handleAsyncRPCResp(r, callback)
	})
}

// Cast 发送消息到目标actor.
func (s *Service) Cast(to ActorUID, msg proto.Message) error {
	payload, err := NewS2SPayload(msg, s.S2S)
	if err != nil {
		return err
	}
	return s.core.Cast(to, &payload)
}
