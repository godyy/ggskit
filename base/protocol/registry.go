package protocol

import (
	"errors"
	"fmt"
	"hash/fnv"
	"reflect"

	"google.golang.org/protobuf/proto"
)

// Registry 用于注册和创建 protobuf 协议结构体.
type Registry struct {
	pid2Type map[PID]reflect.Type
	type2Pid map[reflect.Type]PID
}

// NewRegistry 创建协议注册表.
func NewRegistry() *Registry {
	return &Registry{
		pid2Type: make(map[PID]reflect.Type),
		type2Pid: make(map[reflect.Type]PID),
	}
}

// Register 注册协议类型.
// pid 由消息类型名称哈希生成.
func (r *Registry) Register(msg proto.Message) (PID, error) {
	if msg == nil {
		return 0, errors.New("proto is nil")
	}

	typ := reflect.TypeOf(msg)
	if typ.Kind() != reflect.Ptr {
		return 0, errors.New("proto must be pointer")
	}

	elemTyp := typ.Elem()
	msgName := getMessageName(msg, elemTyp)
	pid := hashPID(msgName)
	if existsTyp, exists := r.pid2Type[pid]; exists {
		if existsTyp != elemTyp {
			return 0, fmt.Errorf("pid %d collision: %s(%s) conflicts with %s", pid, msgName, elemTyp.String(), existsTyp.String())
		}
		return pid, nil
	}
	if existsPid, exists := r.type2Pid[elemTyp]; exists {
		if existsPid != pid {
			return 0, fmt.Errorf("proto type %s already registered with pid %d (want %d)", elemTyp.String(), existsPid, pid)
		}
		return pid, nil
	}

	r.pid2Type[pid] = elemTyp
	r.type2Pid[elemTyp] = pid
	return pid, nil
}

// getMessageName 返回用于计算 PID 的消息名称.
// 优先使用 protobuf 消息全名，避免不同包下同名消息冲突.
func getMessageName(msg proto.Message, elemTyp reflect.Type) string {
	if msgName := string(proto.MessageName(msg)); msgName != "" {
		return msgName
	}
	if pkgPath := elemTyp.PkgPath(); pkgPath != "" {
		return pkgPath + "." + elemTyp.Name()
	}
	return elemTyp.Name()
}

// hashPID 将消息名称稳定映射为 32 位协议 ID.
func hashPID(name string) PID {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(name))
	return hasher.Sum32()
}

// GetPid 通过协议类型获取对象的协议 ID.
func (r *Registry) GetPid(msg proto.Message) (PID, bool) {
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

// Create 通过协议 ID 创建协议实体.
func (r *Registry) Create(pid PID) (proto.Message, error) {
	typ, exists := r.pid2Type[pid]
	if !exists {
		return nil, fmt.Errorf("pid %d not registered", pid)
	}

	inst := reflect.New(typ).Interface().(proto.Message)
	return inst, nil
}

// Check 检查协议 ID 和协议类型是否匹配.
func (r *Registry) Check(pid PID, msg proto.Message) error {
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
