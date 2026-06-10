package cluster

import (
	clusternet "github.com/godyy/gcluster/net"
)

// Config 集群配置.
type Config struct {
	// EtcdEndPoints etcd 节点地址列表.
	EtcdEndPoints []string

	// EtcdRoot 用于发现其它节点信息的etcd根路径.
	EtcdRoot string

	// EtcdWatchPrefix etcd 节点信息变更事件监听前缀.
	EtcdWatchPrefix string

	// Handshake 握手配置.
	Handshake clusternet.HandshakeConfig

	// Session 会话配置.
	Session clusternet.SessionConfig

	// ExpectedConcurrentSessions 预期同时存在的 Session 数量.
	ExpectedConcurrentSessions int
}
