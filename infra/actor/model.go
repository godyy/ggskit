package actor

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

// ModelDirty 带有脏数据的数据模型接口.
type ModelDirty interface {
	Model

	// IsDirty 是否有脏数据.
	IsDirty() (dirty bool, all bool)

	// ClearDirty 清理脏数据.
	ClearDirty()

	// MarshalBSONDirty 序列化脏数据.
	MarshalBSONDirty() ([]byte, error)
}

// ActorWithModel 包含模型的Actor接口.
type ActorWithModel interface {
	Actor

	// GetModel 获取模型实例.
	GetModel() Model

	// OnModelDirty 通知模型存在脏数据.
	OnModelDirty()
}
