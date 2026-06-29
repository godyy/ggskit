package actor

import (
	"github.com/godyy/gactor"
	"github.com/godyy/ggskit/base/protocol"
)

// HandlerFunc
type HandlerFunc = gactor.HandlerFunc

// HandlerMap 处理函数映射.
type HandlerMap map[protocol.PID]HandlerFunc

// Register 注册处理函数.
func (hm HandlerMap) Register(pid protocol.PID, funcs ...HandlerFunc) bool {
	if _, ok := hm[pid]; ok {
		return false
	}
	hm[pid] = gactor.NewHandlersChain(funcs...).Handle
	return true
}
