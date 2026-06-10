package c2s

import (
	"errors"
	"fmt"

	pkgerrors "github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// ProtoRegistry 提供 C2S 编解码所需的协议注册能力.
type ProtoRegistry interface {
	GetPid(msg proto.Message) (uint16, bool)
	Create(pid uint16) (proto.Message, error)
}

// DecodeMessage 解码数据包中的消息.
func DecodeMessage(reg ProtoRegistry, p []byte) (proto.Message, error) {
	if len(p) < HeadLen {
		return nil, errors.New("packet length is too short")
	}

	pid := HeadGetPid(p)
	msg, err := reg.Create(pid)
	if err != nil {
		return nil, err
	}

	if err = proto.Unmarshal(p[HeadLen:], msg); err != nil {
		return nil, pkgerrors.WithMessagef(err, "unmarshal proto pid=%d", pid)
	}

	return msg, nil
}

// EncodePacket 编码数据包.
func EncodePacket(reg ProtoRegistry, pt int8, seq uint32, msg proto.Message) (p []byte, err error) {
	pid, exists := reg.GetPid(msg)
	if !exists {
		return nil, fmt.Errorf("pid of %s not found", proto.MessageName(msg))
	}

	protoBytes, err := proto.Marshal(msg)
	if err != nil {
		return nil, pkgerrors.WithMessagef(err, "marshal proto pid=%d", pid)
	}

	p = make([]byte, HeadLen+len(protoBytes))
	HeadSetPt(p, pt)
	HeadSetSeq(p, seq)
	HeadSetPid(p, pid)
	copy(p[HeadLen:], protoBytes)

	return p, nil
}

// EncodeReqPacket 编码请求数据包.
func EncodeReqPacket(reg ProtoRegistry, seq uint32, msg proto.Message) ([]byte, error) {
	return EncodePacket(reg, PtReq, seq, msg)
}

// EncodeRespPacket 编码响应数据包.
func EncodeRespPacket(reg ProtoRegistry, seq uint32, msg proto.Message) ([]byte, error) {
	return EncodePacket(reg, PtResp, seq, msg)
}

// EncodePushPacket 编码推送数据包.
func EncodePushPacket(reg ProtoRegistry, msg proto.Message) ([]byte, error) {
	return EncodePacket(reg, PtPush, 0, msg)
}
