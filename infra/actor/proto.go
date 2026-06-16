package actor

import "github.com/godyy/ggskit/base/protocol"

// ProtoRegistry 供 Actor 框架使用的综合协议注册表.
type ProtoRegistry struct {
	C2S *protocol.Registry
	S2S *protocol.Registry
}

func NewProtoRegistry() *ProtoRegistry {
	return &ProtoRegistry{
		C2S: protocol.NewRegistry(),
		S2S: protocol.NewRegistry(),
	}
}
