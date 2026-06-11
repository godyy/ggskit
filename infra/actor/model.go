package actor

import "go.mongodb.org/mongo-driver/v2/bson"

// ActorWithModel 包含模型的Actor接口.
type ActorWithModel interface {
	Actor

	// GetModel 获取模型实例.
	GetModel() Model

	// OnModelDirty model脏事件.
	OnModelDirty()
}

// Model 数据模型接口.
type Model interface {
	// GetHashKey 获取Model的哈希键.
	GetHashKey() any

	// GetCollection 存储Model的集合名称.
	GetCollection() string

	// GetFilter 获取Model的查询过滤器.
	GetFilter() any

	// Release 释放模型资源.
	Release()
}

// ModelWithDirty 带有脏数据的数据模型接口.
type ModelWithDirty interface {
	Model

	// IsDirty 是否有脏数据.
	IsDirty() (dirty bool, all bool)

	// ClearDirty 清理脏数据.
	ClearDirty()

	// MarshalBSONDirty 序列化脏数据.
	MarshalBSONDirty() ([]byte, error)
}

// ModelDirty 脏数据模型.
type ModelDirty struct {
	actor   ActorWithModel // 关联ActorWithModel.
	dirties bson.M         // 脏数据
	all     bool           // 是否全脏位.
}

// NewModelDirty 构造脏数据模型.
func NewModelDirty(actor ActorWithModel) *ModelDirty {
	return &ModelDirty{actor: actor}
}

// SetDirty 设置脏数据.
func (md *ModelDirty) SetDirty(key string, value any) {
	if md.dirties == nil {
		md.dirties = make(bson.M)
	}
	md.dirties[key] = value
	md.actor.OnModelDirty()
}

// SetDirtyAll 设置全脏位.
func (md *ModelDirty) SetDirtyAll() {
	md.all = true
	md.actor.OnModelDirty()
}

// IsDirty 是否有脏数据.
func (md *ModelDirty) IsDirty() (dirty bool, all bool) {
	all = md.all
	dirty = all || len(md.dirties) > 0
	return
}

// ClearDirty 清除脏数据.
func (md *ModelDirty) ClearDirty() {
	md.dirties = nil
	md.all = false
}

// MarshalBSONDirty 序列化脏数据.
func (md *ModelDirty) MarshalBSONDirty() ([]byte, error) {
	return bson.Marshal(md.dirties)
}

func (md *ModelDirty) Release() {
	md.actor = nil
	md.dirties = nil
}

// ModelDirtyAll 全脏脏数据模型.
type ModelDirtyAll struct {
	actor ActorWithModel // 关联ActorWithModel.
	dirty bool           // 是否脏位.
}

// SetDirty 设置脏位.
func (md *ModelDirtyAll) SetDirty() {
	md.dirty = true
	md.actor.OnModelDirty()
}

// IsDirty 是否有脏数据.
func (md *ModelDirtyAll) IsDirty() (dirty bool, all bool) {
	return md.dirty, md.dirty
}

// ClearDirty 清除脏数据.
func (md *ModelDirtyAll) ClearDirty() {
	md.dirty = false
}

// MarshalBSONDirty 序列化脏数据.
func (md *ModelDirtyAll) MarshalBSONDirty() ([]byte, error) {
	return bson.Marshal(md.actor.GetModel())
}
