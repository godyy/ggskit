package actor

import (
	"github.com/godyy/gactor"
	"github.com/godyy/glog"
)

// ClientConfig Actor客户端配置.
type ClientConfig struct {
	// Core 核心配置.
	Core *gactor.ClientConfig

	// Logger 日志记录器.
	Logger glog.Logger
}

// Client 声明为gactor.Client.
type Client = gactor.Client

// NewClient 创建Actor客户端.
func NewClient(cfg *ClientConfig) *Client {
	return gactor.NewClient(cfg.Core, gactor.WithClientLogger(cfg.Logger))
}
