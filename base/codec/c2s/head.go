package c2s

import (
	"encoding/binary"
)

const (
	headPtLen  = 1                                   // 包头数据包类型长度
	headSeqLen = 4                                   // 包头数据包序号长度
	headPidLen = 2                                   // 包头数据包协议ID长度
	HeadLen    = headPtLen + headSeqLen + headPidLen // 包头长度

	headSeqFirst = headPtLen
	headPidFirst = headSeqFirst + headSeqLen
)

const (
	PtReq  = 1 // 请求数据包类型
	PtResp = 2 // 响应数据包类型
	PtPush = 3 // 推送数据包类型
)

func CheckPt(pt int8) bool {
	return pt >= PtReq && pt <= PtPush
}

func CheckPtC2S(pt int8) bool {
	return pt == PtReq
}

func CheckPtS2C(pt int8) bool {
	return pt >= PtResp && pt <= PtPush
}

func HeadSetPt(p []byte, pt int8) {
	p[0] = byte(pt)
}

func HeadGetPt(p []byte) int8 {
	return int8(p[0])
}

func HeadSetSeq(p []byte, seq uint32) {
	binary.BigEndian.PutUint32(p[headSeqFirst:headPidFirst], seq)
}

func HeadGetSeq(p []byte) uint32 {
	return binary.BigEndian.Uint32(p[headSeqFirst:headPidFirst])
}

func HeadSetPid(p []byte, pid uint16) {
	binary.BigEndian.PutUint16(p[headPidFirst:], pid)
}

func HeadGetPid(p []byte) uint16 {
	return binary.BigEndian.Uint16(p[headPidFirst:HeadLen])
}

// Head 数据包包头.
type Head [HeadLen]byte

// GetPt 获取数据包类型.
func (h *Head) GetPt() int8 {
	return HeadGetPt(h[:])
}

// SetPt 设置数据包类型.
func (h *Head) SetPt(pt int8) {
	HeadSetPt(h[:], pt)
}

// GetSeq 获取数据包序号.
func (h *Head) GetSeq() uint32 {
	return HeadGetSeq(h[:])
}

// SetSeq 设置数据包序号.
func (h *Head) SetSeq(seq uint32) {
	HeadSetSeq(h[:], seq)
}

// GetPid 获取协议ID.
func (h *Head) GetPid() uint16 {
	return HeadGetPid(h[:])
}

// SetPid 设置协议ID.
func (h *Head) SetPid(pid uint16) {
	HeadSetPid(h[:], pid)
}

func NewHead(pt int8, seq uint32, pid uint16) Head {
	h := Head{}
	h.SetPt(pt)
	h.SetSeq(seq)
	h.SetPid(pid)
	return h
}
