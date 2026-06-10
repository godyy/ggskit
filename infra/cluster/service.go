package cluster

import (
	"context"
	"net"
	"time"

	"github.com/godyy/gcluster"
	clusternet "github.com/godyy/gcluster/net"
	"github.com/godyy/glog"
)

// ServiceConfig 集群服务配置.
type ServiceConfig struct {
	// Core 核心配置.
	Core *Config

	// Self 本地节点信息.
	Self *Node

	// CenterListener 中心监听器.
	CenterListener CenterListener

	// Handler 集群代理处理函数.
	Handler gcluster.AgentHandler

	// Logger 日志记录器.
	Logger glog.Logger

	// DefCtxTimeout 默认上下文超时时间.
	DefCtxTimeout time.Duration
}

// Service 集群服务.
type Service struct {
	center *Center         // 数据中心.
	agent  *gcluster.Agent // 集群代理.
}

// NewService 构造集群服务.
func NewService(cfg *ServiceConfig) (*Service, error) {
	// 创建center
	center, err := NewCenter(&CenterConfig{
		EndPoints:   cfg.Core.EtcdEndPoints,
		Root:        cfg.Core.EtcdRoot,
		WatchPrefix: cfg.Core.EtcdWatchPrefix,
		Self:        cfg.Self,
		Listener:    cfg.CenterListener,
		Log:         cfg.Logger.Named("cluster-center"),
	})
	if err != nil {
		return nil, err
	}

	// 创建agent
	agent, err := gcluster.CreateAgent(
		&gcluster.AgentConfig{
			Center: center,
			Net: &clusternet.ServiceConfig{
				NodeId:    cfg.Self.GetNodeId(),
				Addr:      cfg.Self.Addr,
				Handshake: cfg.Core.Handshake,
				Session:   cfg.Core.Session,
				Dialer: func(addr string) (net.Conn, error) {
					return net.Dial("tcp", addr)
				},
				ListenerCreator: func(addr string) (net.Listener, error) {
					return net.Listen("tcp", addr)
				},
				TimerSystem:                clusternet.NewTimerHeap(),
				ExpectedConcurrentSessions: cfg.Core.ExpectedConcurrentSessions,
			},
			Handler: cfg.Handler,
		},
		gcluster.WithLogger(cfg.Logger),
		gcluster.WithServiceOptions(clusternet.WithServiceLogger(cfg.Logger)),
	)
	if err != nil {
		return nil, err
	}

	return &Service{
		center: center,
		agent:  agent,
	}, nil
}

// Start 启动集群服务.
func (s *Service) Start() error {
	// 启动agent
	if err := s.agent.Start(); err != nil {
		return err
	}

	// 启动center
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := s.center.Start(ctx); err != nil {
		return err
	}

	return nil
}

// Start 关闭集群服务.
func (s *Service) Stop() {
	// 关闭center
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	s.center.Close(ctx)

	// 关闭agent
	s.agent.Close()
}

// Send2Node 发送字节数据 b 到 nodeId 指定的节点.
func (s *Service) Send2Node(nodeId string, b []byte) error {
	return s.agent.Send2Node(nodeId, b)
}
