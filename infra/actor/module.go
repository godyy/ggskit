package actor

import (
	"reflect"

	pkgerrors "github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ModelWithModule 模块模型接口.
type ModelWithModule interface {
	ModelDirty

	// GetModule 获取模块实例.
	GetModule(mk ModuleKey, autoCreate bool) Module
}

// ActorWithModule 包含数据模块的Actor接口.
type ActorWithModule interface {
	ActorWithModel

	// GetModelWithModule 获取模块模型实例.
	GetModelWithModule() ModelWithModule
}

// ModuleKey 模块关键字接口
type ModuleKey interface {
	ModuleKey() string
}

// Module 数据模块
type Module interface {
	ModuleKey

	// OnInit 初始化模块, 在模块被创建时调用.
	OnInit()
}

// ModuleSingle 单值模块.
// 用于存储单值模块数据.
type ModuleSingle[V any, Key ModuleKey] struct {
	value V
}

func (m *ModuleSingle[V, Key]) OnInit() {}

func (m *ModuleSingle[V, Key]) ModuleKey() string {
	var k Key
	return k.ModuleKey()
}

func (m *ModuleSingle[V, Key]) Get() V {
	return m.value
}

func (m *ModuleSingle[V, Key]) Set(v V) {
	m.value = v
}

func (m *ModuleSingle[V, Key]) MarshalBSONValue() (byte, []byte, error) {
	t, bytes, err := bson.MarshalValue(m.value)
	return byte(t), bytes, err
}

func (m *ModuleSingle[V, Key]) UnmarshalBSONValue(t byte, data []byte) error {
	return bson.UnmarshalValue(bson.Type(t), data, &m.value)
}

// Modules Module集中管理器.
type Modules struct {
	moduleRegistry *ModuleRegistry   // 模块注册表
	modules        map[string]Module // 模块实例映射表
}

func NewModules(moduleRegistry *ModuleRegistry) *Modules {
	return &Modules{
		moduleRegistry: moduleRegistry,
		modules:        make(map[string]Module, moduleRegistry.Len()),
	}
}

// InitAllModules 初始化说有模块实例
func (ms *Modules) InitAllModules() {
	moduleRegistry := ms.moduleRegistry
	for _, mi := range moduleRegistry.moduleList {
		m := mi.create()
		ms.modules[mi.key] = m
	}
}

// GetModule 获取模块实例
func (ms *Modules) GetModule(mk ModuleKey, autoCreate bool) Module {
	m := ms.modules[mk.ModuleKey()]
	if m == nil && autoCreate {
		m = ms.moduleRegistry.Create(mk)
		ms.modules[mk.ModuleKey()] = m
	}

	return m
}

// Release 清理所有模块实例, 解除引用model.
// 释放时调用.
func (ms *Modules) Release() {
	ms.moduleRegistry = nil
	ms.modules = nil
}

// MarshalBSON 序列化模块实例BSON
func (ms *Modules) MarshalBSON() ([]byte, error) {
	moduleRegistry := ms.moduleRegistry
	elements := make(bson.D, 0, len(ms.modules))
	for _, mi := range moduleRegistry.moduleList {
		module := ms.modules[mi.key]
		if module == nil {
			continue
		}
		elements = append(elements, bson.E{Key: mi.key, Value: module})
	}
	return bson.Marshal(elements)
}

// UnmarshalBSON 反序列化模块实例BSON
func (ms *Modules) UnmarshalBSON(data []byte) error {
	raw := bson.Raw(data)
	moduleRegistry := ms.moduleRegistry
	for _, mi := range moduleRegistry.moduleList {
		value := raw.Lookup(mi.key)
		if value.IsZero() {
			continue
		}
		m := mi.create()
		if err := bson.UnmarshalValue(value.Type, value.Value, m); err != nil {
			return pkgerrors.WithMessagef(err, "unmarshal module %s", mi.key)
		}
		ms.modules[mi.key] = m
	}
	return nil
}

// moduleInfo 模块信息
type moduleInfo struct {
	key string       // key
	typ reflect.Type // typ
}

func (mi *moduleInfo) create() Module {
	m := reflect.New(mi.typ).Interface().(Module)
	m.OnInit()
	return m
}

// ModuleRegistry 模块注册表
type ModuleRegistry struct {
	moduleList []*moduleInfo          // 模块列表, 模块会按照注册的顺序序列化
	moduleMap  map[string]*moduleInfo // 模块映射表
}

func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{
		moduleMap: make(map[string]*moduleInfo),
	}
}

func (mr *ModuleRegistry) Len() int {
	return len(mr.moduleList)
}

// Register 注册模块
func (mr *ModuleRegistry) Register(m Module) *ModuleRegistry {
	if _, ok := mr.moduleMap[m.ModuleKey()]; ok {
		panic("module " + m.ModuleKey() + " already registered")
	}
	mt := reflect.TypeOf(m)
	if mt.Kind() != reflect.Ptr {
		panic("module " + m.ModuleKey() + " must be a pointer")
	}
	mt = mt.Elem()
	mi := &moduleInfo{
		key: m.ModuleKey(),
		typ: mt,
	}
	mr.moduleList = append(mr.moduleList, mi)
	mr.moduleMap[mi.key] = mi
	return mr
}

// Create 创建模块实例
func (mr *ModuleRegistry) Create(mk ModuleKey) Module {
	if mi := mr.moduleMap[mk.ModuleKey()]; mi != nil {
		return mi.create()
	}
	return nil
}

// RegisterModule 注册模块的泛型封装.
func RegisterModule[M Module](mr *ModuleRegistry) {
	var m M
	mr.Register(m)
}

// GetModule 获取容器中的模块实例的泛型封装.
func GetModule[M Module](model ModelWithModule, autoCreate bool) (m M) {
	module := model.GetModule(m, autoCreate)
	if module == nil {
		panic("module " + m.ModuleKey() + " not exists")
	}
	moduleM, ok := module.(M)
	if !ok {
		panic("module " + m.ModuleKey() + " type is " + reflect.TypeOf(moduleM).Name())
	}
	return moduleM
}

// GetActorModule 通过actor获取模块的通用泛型封装.
func GetActorModule[M Module](actor ActorWithModule, autoCreate bool) M {
	return GetModule[M](actor.GetModelWithModule(), autoCreate)
}
