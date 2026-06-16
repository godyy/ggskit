package actor

import "github.com/godyy/gactor"

// HandlerFunc
type HandlerFunc = gactor.HandlerFunc

// HandlerMap 处理函数映射.
type HandlerMap map[uint16]HandlerFunc

// Register 注册处理函数.
func (hm HandlerMap) Register(pid uint16, funcs ...HandlerFunc) bool {
	if _, ok := hm[pid]; ok {
		return false
	}
	hm[pid] = gactor.NewHandlersChain(funcs...).Handle
	return true
}
