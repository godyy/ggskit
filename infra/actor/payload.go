package actor

import (
	"fmt"

	codecc2s "github.com/godyy/ggskit/base/codec/c2s"
	codecs2s "github.com/godyy/ggskit/base/codec/s2s"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// C2SPayload C2S负载数据.
type C2SPayload struct {
	Pt  int8          // 数据包类型
	Seq uint32        // 序号
	PID uint16        // 协议ID
	Msg proto.Message // 携带的消息
}

// S2SPayload S2S负载数据.
type S2SPayload struct {
	PID uint16        // 协议ID
	Msg proto.Message // 携带的消息
}

// NewC2SPayload 创建C2S负载数据.
func NewC2SPayload(pt int8, seq uint32, msg proto.Message, protoReg codecc2s.ProtoRegistry) (payload C2SPayload, err error) {
	if msg == nil {
		err = errors.New("msg is nil")
		return
	}
	if protoReg == nil {
		err = errors.New("protoReg is nil")
		return
	}

	pid, ok := protoReg.GetPid(msg)
	if !ok {
		err = fmt.Errorf("msg %T not registered", msg)
		return
	}

	payload.Pt = pt
	payload.Seq = seq
	payload.PID = pid
	payload.Msg = msg
	return
}

// NewS2SPayload 创建S2S负载数据.
func NewS2SPayload(msg proto.Message, protoReg codecs2s.ProtoRegistry) (payload S2SPayload, err error) {
	if msg == nil {
		err = errors.New("msg is nil")
		return
	}
	if protoReg == nil {
		err = errors.New("protoReg is nil")
		return
	}

	pid, ok := protoReg.GetPid(msg)
	if !ok {
		err = fmt.Errorf("msg %T not registered", msg)
		return
	}

	payload.PID = pid
	payload.Msg = msg
	return
}
