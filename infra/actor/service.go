package actor

import (
	"context"
	"errors"
	"fmt"

	"github.com/godyy/gactor"
	codecs2s "github.com/godyy/ggskit/base/codec/s2s"
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
	S2SProtoReg codecs2s.ProtoRegistry
}

// Service 封装gactor.Service.
type Service struct {
	core        *gactor.Service
	s2sProtoReg codecs2s.ProtoRegistry
}

// NewService 创建Actor服务.
func NewService(cfg *ServiceConfig) (*Service, error) {
	if cfg == nil {
		return nil, errors.New("service config is nil")
	}
	if cfg.S2SProtoReg == nil {
		return nil, errors.New("s2s proto reg is nil")
	}
	return &Service{
		core:        gactor.NewService(cfg.Core, gactor.WithServiceLogger(cfg.Logger)),
		s2sProtoReg: cfg.S2SProtoReg,
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
func (s *Service) StartActor(ctx context.Context, uid gactor.ActorUID) error {
	return s.core.StartActor(ctx, uid)
}

// RPC 同步RPC调用.
func (s *Service) RPC(ctx context.Context, to gactor.ActorUID, args proto.Message) (proto.Message, error) {
	pid, ok := s.s2sProtoReg.GetPid(args)
	if !ok {
		return nil, fmt.Errorf("args %T not registered", args)
	}

	var (
		argsPayload  S2SPayload
		replyPayload S2SPayload
	)
	argsPayload.PID = pid
	argsPayload.Msg = args

	if err := s.core.RPCWithContext(ctx, to, &argsPayload, &replyPayload); err != nil {
		return nil, err
	}

	return replyPayload.Msg, nil
}

// AsyncRPC 异步RPC调用.
func (s *Service) AsyncRPC(ctx context.Context, to gactor.ActorUID, args proto.Message, callback func(reply proto.Message, err error)) error {
	pid, ok := s.s2sProtoReg.GetPid(args)
	if !ok {
		return fmt.Errorf("args %T not registered", args)
	}

	var (
		argsPayload S2SPayload
	)
	argsPayload.PID = pid
	argsPayload.Msg = args

	if err := s.core.AsyncRPCWithContext(ctx, to, &argsPayload, func(r *gactor.RPCResp) {
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
	}); err != nil {
		return err
	}

	return nil
}

// Cast 发送消息到目标actor.
func (s *Service) Cast(ctx context.Context, to gactor.ActorUID, msg proto.Message) error {
	pid, ok := s.s2sProtoReg.GetPid(msg)
	if !ok {
		return fmt.Errorf("msg %T not registered", msg)
	}
	payload := S2SPayload{
		PID: pid,
		Msg: msg,
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.core.Cast(to, &payload)
}
