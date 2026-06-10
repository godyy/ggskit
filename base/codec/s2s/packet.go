package s2s

import (
	"encoding/binary"
	"fmt"

	pkgerrors "github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// ProtoRegistry 提供 S2S 编解码所需的协议注册能力.
type ProtoRegistry interface {
	GetPid(msg proto.Message) (uint16, bool)
	Create(pid uint16) (proto.Message, error)
	Check(pid uint16, msg proto.Message) error
}

// EncodePayload 编码负载数据.
func EncodePayload(reg ProtoRegistry, pid uint16, msg proto.Message) ([]byte, error) {
	if err := reg.Check(pid, msg); err != nil {
		return nil, err
	}
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return nil, pkgerrors.WithMessagef(err, "marshal msg of pid %d failed", pid)
	}
	payload := make([]byte, 2+len(msgBytes))
	binary.BigEndian.PutUint16(payload, pid)
	copy(payload[2:], msgBytes)
	return payload, nil
}

// DecodePayload 解码负载数据.
func DecodePayload(reg ProtoRegistry, p []byte) (uint16, proto.Message, error) {
	if len(p) < 2 {
		return 0, nil, fmt.Errorf("payload len %d must > %d", len(p), 2)
	}
	pid := binary.BigEndian.Uint16(p)
	msg, err := reg.Create(pid)
	if err != nil {
		return 0, nil, pkgerrors.WithMessagef(err, "msg of pid %d not registered", pid)
	}
	if err := proto.Unmarshal(p[2:], msg); err != nil {
		return 0, nil, pkgerrors.WithMessagef(err, "unmarshal msg of pid %d failed", pid)
	}
	return pid, msg, nil
}
