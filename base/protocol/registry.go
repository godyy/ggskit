package protocol

import (
	"errors"
	"fmt"
	"reflect"

	"google.golang.org/protobuf/proto"
)

// Registry 用于注册和创建 protobuf 协议结构体.
type Registry struct {
	pid2Type map[uint16]reflect.Type
	type2Pid map[reflect.Type]uint16
	name2Pid map[string]uint16
}

// NewRegistry 创建协议注册表.
func NewRegistry() *Registry {
	return &Registry{
		pid2Type: make(map[uint16]reflect.Type),
		type2Pid: make(map[reflect.Type]uint16),
		name2Pid: make(map[string]uint16),
	}
}

// Register 注册协议类型.
func (r *Registry) Register(pid uint16, msg proto.Message) error {
	if msg == nil {
		return errors.New("proto is nil")
	}

	typ := reflect.TypeOf(msg)
	if typ.Kind() != reflect.Ptr {
		return errors.New("proto must be pointer")
	}

	if _, exists := r.pid2Type[pid]; exists {
		return fmt.Errorf("pid %d already registered", pid)
	}

	elemTyp := typ.Elem()
	r.pid2Type[pid] = elemTyp
	r.type2Pid[elemTyp] = pid
	r.name2Pid[elemTyp.Name()] = pid
	return nil
}

// GetPid 通过协议类型获取对象的协议 ID.
func (r *Registry) GetPid(msg proto.Message) (uint16, bool) {
	typ := reflect.TypeOf(msg)
	if typ.Kind() != reflect.Ptr {
		return 0, false
	}

	elemTyp := typ.Elem()
	pid, exists := r.type2Pid[elemTyp]
	if !exists {
		return 0, false
	}

	return pid, true
}

// GetPidByName 通过协议类型名称获取协议 ID.
func (r *Registry) GetPidByName(name string) (uint16, bool) {
	pid, exists := r.name2Pid[name]
	if !exists {
		return 0, false
	}

	return pid, true
}

// Create 通过协议 ID 创建协议实体.
func (r *Registry) Create(pid uint16) (proto.Message, error) {
	typ, exists := r.pid2Type[pid]
	if !exists {
		return nil, fmt.Errorf("pid %d not registered", pid)
	}

	inst := reflect.New(typ).Interface().(proto.Message)
	return inst, nil
}

// CreateByName 通过协议类型名称创建协议实体.
func (r *Registry) CreateByName(name string) (proto.Message, uint16, error) {
	pid, exists := r.name2Pid[name]
	if !exists {
		return nil, 0, fmt.Errorf("%s not registered", name)
	}

	return reflect.New(r.pid2Type[pid]).Interface().(proto.Message), pid, nil
}

// Check 检查协议 ID 和协议类型是否匹配.
func (r *Registry) Check(pid uint16, msg proto.Message) error {
	pidTyp, exists := r.pid2Type[pid]
	if !exists {
		return fmt.Errorf("pid %d not registered", pid)
	}

	msgTyp := reflect.TypeOf(msg)
	if msgTyp.Kind() != reflect.Ptr {
		return errors.New("proto must be pointer")
	}
	msgTyp = msgTyp.Elem()

	if msgTyp != pidTyp {
		return errors.New("proto type not match")
	}

	return nil
}
