package actor

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/godyy/gactor"
	codecc2s "github.com/godyy/ggskit/base/codec/c2s"
	codecs2s "github.com/godyy/ggskit/base/codec/s2s"
	pkgerrors "github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// CodecConfig Actor 数据包编解码器配置.
type CodecConfig struct {
	C2SProtoReg codecc2s.ProtoRegistry
	S2SProtoReg codecs2s.ProtoRegistry
}

// Codec Actor 数据包编解码器.
type Codec struct {
	c2sProtoReg codecc2s.ProtoRegistry
	s2sProtoReg codecs2s.ProtoRegistry
}

// NewCodec 创建 Actor 数据包编解码器.
func NewCodec(cfg *CodecConfig) (*Codec, error) {
	if cfg == nil {
		return nil, errors.New("codec config is nil")
	}
	if cfg.C2SProtoReg == nil {
		return nil, errors.New("c2s proto reg is nil")
	}
	if cfg.S2SProtoReg == nil {
		return nil, errors.New("s2s proto reg is nil")
	}
	return &Codec{
		c2sProtoReg: cfg.C2SProtoReg,
		s2sProtoReg: cfg.S2SProtoReg,
	}, nil
}

// GetBytes 获取指定容量的字节切片.
func (c *Codec) GetBytes(cap int) []byte {
	return make([]byte, 0, cap)
}

// PutBytes 回收字节切片.
func (c *Codec) PutBytes(b []byte) {
}

// Encode 编码数据包.
// allocator 提供了获取数据包类型和分配数据包切片的功能.
// 根据数据包类型编码 payload, 然后调用 allocator 分配
// 数据包切片, 将编码后的 payload 数据写入数据包切片中.
// 数据包类型包括:
//
//	PacketTypeRawResp, PacketTypeRawPush
//	PacketTypeS2SRpc, PacketTypeS2SRpcResp, PacketTypeS2SCast
func (c *Codec) Encode(allocator gactor.PacketAllocator, payload any) ([]byte, error) {
	var buffer gactor.Buffer
	switch allocator.PacketType() {
	case gactor.PacketTypeRawResp, gactor.PacketTypeRawPush:
		p := payload.(*C2SPayload)
		pid, ok := c.c2sProtoReg.GetPid(p.Msg)
		if !ok {
			return nil, fmt.Errorf("msg %v not registered", reflect.TypeOf(p.Msg))
		}
		msgBytes, err := proto.Marshal(p.Msg)
		if err != nil {
			return nil, pkgerrors.WithMessagef(err, "marshal msg of pid %d failed", pid)
		}
		head := codecc2s.NewHead(p.Pt, p.Seq, pid)
		if err := allocator.AllocBuf(&buffer, len(head)+len(msgBytes)); err != nil {
			return nil, pkgerrors.WithMessagef(err, "alloc buf failed")
		}
		if _, err := buffer.Write(head[:]); err != nil {
			return nil, pkgerrors.WithMessage(err, "write head to buf failed")
		}
		if _, err := buffer.Write(msgBytes); err != nil {
			return nil, pkgerrors.WithMessage(err, "write msg to buf failed")
		}

	case gactor.PacketTypeS2SRpc, gactor.PacketTypeS2SRpcResp, gactor.PacketTypeS2SCast:
		p := payload.(*S2SPayload)
		msgBytes, err := proto.Marshal(p.Msg)
		if err != nil {
			return nil, pkgerrors.WithMessagef(err, "marshal msg of pid %d failed", p.PID)
		}
		if err := allocator.AllocBuf(&buffer, 2+len(msgBytes)); err != nil {
			return nil, pkgerrors.WithMessagef(err, "alloc buf failed")
		}
		if err := buffer.WriteUint16(p.PID); err != nil {
			return nil, pkgerrors.WithMessage(err, "write pid to buf failed")
		}
		if _, err := buffer.Write(msgBytes); err != nil {
			return nil, pkgerrors.WithMessage(err, "write msg to buf failed")
		}

	default:
		return nil, errors.New("not implemented")
	}

	defer buffer.SetBuf(nil)

	return buffer.Data(), nil
}

// EncodePayload 编码负载数据.
// 根据数据包类型 pt 编码 payload 并生成数据切片返回.
// 数据包类型包括:
//
//	PacketTypeS2SRpc, PacketTypeS2SCast
func (c *Codec) EncodePayload(pt gactor.PacketType, payload any) ([]byte, error) {
	p := payload.(*S2SPayload)
	return codecs2s.EncodePayload(c.s2sProtoReg, p.PID, p.Msg)
}

// DecodePayload 解码负载数据.
// 根据数据包类型 pt 解码 b 中负载数据并填充入 v 指向的对象中.
// 数据包类型包括:
//
//	PacketTypeRawReq
//	PacketTypeS2SRpc, PacketTypeS2SRpcResp, PacketTypeS2SCast
//
// 返回 ErrBytesEscape, 表示 b 中的数据切片被外部劫持, 系统
// 内部将不再自动回收数据切片.
func (c *Codec) DecodePayload(pt gactor.PacketType, b *gactor.Buffer, payload any) error {
	switch pt {
	case gactor.PacketTypeRawReq:
		p := payload.(*C2SPayload)
		var head codecc2s.Head
		if _, err := b.Read(head[:]); err != nil {
			return pkgerrors.WithMessage(err, "read head from buf failed")
		}
		p.Pt = head.GetPt()
		p.Seq = head.GetSeq()
		p.PID = head.GetPid()
		msg, err := c.c2sProtoReg.Create(p.PID)
		if err != nil {
			return pkgerrors.WithMessagef(err, "msg of pid %d not registered", p.PID)
		}
		if err := proto.Unmarshal(b.UnreadData(), msg); err != nil {
			return pkgerrors.WithMessagef(err, "unmarshal msg of pid %d failed", p.PID)
		}
		p.Msg = msg

	case gactor.PacketTypeS2SRpc, gactor.PacketTypeS2SRpcResp, gactor.PacketTypeS2SCast:
		p := payload.(*S2SPayload)
		pid, msg, err := codecs2s.DecodePayload(c.s2sProtoReg, b.UnreadData())
		if err != nil {
			return err
		}
		p.PID = pid
		p.Msg = msg

	default:
		return errors.New("not implemented")
	}

	return nil
}
