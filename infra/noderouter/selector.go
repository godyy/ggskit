package noderouter

// UpdateOpType 更新操作类型.
type UpdateOpType int8

const (
	UpdateAdd    UpdateOpType = iota // 添加节点
	UpdateRemove                     // 移除节点
)

// UpdateOp 单个更新操作：添加或移除若干节点ID.
type UpdateOp struct {
	Type UpdateOpType
	IDs  []string
}

// Selector 节点路由选择器接口.
type Selector interface {
	// Set 设置节点集合：
	//  - 当 all=true 时，表示全量替换所有分组；未出现的分组将被移除
	//  - 当 all=false 时，仅替换传入的分组，其它分组保持不变
	Set(groups map[string][]string, all bool)
	// Update 批量有序更新多个分组的节点集合；调用方需保证同一分组内操作顺序正确.
	Update(updates map[string][]UpdateOp)
	// Pick 根据 key 选择前 n 个候选节点ID；当 n<=1 时返回单个候选；未知组或无候选返回空切片.
	Pick(group string, key []byte, n int) []string
	// Has 判断节点 ID 当前是否存在于路由集合中.
	Has(nodeID string) bool
}
