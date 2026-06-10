package actor

import (
	"encoding/binary"
	"fmt"

	"github.com/godyy/gactor"
	"github.com/godyy/ggskit/infra/cluster"
	"github.com/godyy/ggskit/infra/noderouter"
)

// NodeGroupFunc 返回节点所属分组。
// ok=false 表示该节点不应参与当前路由。
type NodeGroupFunc func(node *cluster.Node) (group string, ok bool)

// ActorNodeGroupFunc 返回 Actor 所属的部署节点分组。
// ok=false 表示该 Actor 类别当前不支持路由。
type ActorNodeGroupFunc func(uid gactor.ActorUID) (group string, ok bool)

// ActorFixedNodeFunc 返回 Actor 固定部署节点。
// ok=false 表示该 Actor 不使用固定节点路由。
type ActorFixedNodeFunc func(uid gactor.ActorUID) (nodeID string, ok bool)

// RouterConfig Router 配置.
type RouterConfig struct {
	Selector       noderouter.Selector
	NodeGroup      NodeGroupFunc
	ActorFixedNode ActorFixedNodeFunc
	ActorNodeGroup ActorNodeGroupFunc
}

// Router Actor 节点路由器.
// 仅用于在 Actor 尚未注册时，基于节点类别分组选择合适部署节点。
type Router struct {
	selector       noderouter.Selector
	nodeGroup      NodeGroupFunc
	actorFixedNode ActorFixedNodeFunc
	actorNodeGroup ActorNodeGroupFunc
}

// NewRouter 创建 Actor 节点路由器.
func NewRouter(cfg RouterConfig) (*Router, error) {
	if cfg.NodeGroup == nil {
		return nil, fmt.Errorf("router node group func is required")
	}
	if cfg.ActorFixedNode == nil {
		return nil, fmt.Errorf("router actor fixed node func is required")
	}
	if cfg.ActorNodeGroup == nil {
		return nil, fmt.Errorf("router actor node group func is required")
	}

	selector := cfg.Selector
	if selector == nil {
		selector = noderouter.NewRendezvousSelector()
	}
	return &Router{
		selector:       selector,
		nodeGroup:      cfg.NodeGroup,
		actorFixedNode: cfg.ActorFixedNode,
		actorNodeGroup: cfg.ActorNodeGroup,
	}, nil
}

func actorUIDRouteKey(uid gactor.ActorUID) []byte {
	var key [10]byte
	binary.BigEndian.PutUint16(key[:2], uid.Category)
	binary.BigEndian.PutUint64(key[2:], uint64(uid.ID))
	return key[:]
}

// SetNodes 设置可用于部署 Actor 的节点集合.
func (r *Router) SetNodes(nodes []*cluster.Node, all bool) {
	groups := make(map[string][]string)
	for _, node := range nodes {
		if node == nil {
			continue
		}
		group, ok := r.nodeGroup(node)
		if !ok || group == "" {
			continue
		}
		nodeID := node.GetNodeId()
		groups[group] = append(groups[group], nodeID)
	}
	r.selector.Set(groups, all)
}

// UpdateEvents 增量更新可用于部署 Actor 的节点集合.
func (r *Router) UpdateEvents(events []cluster.NodeEvent) {
	updates := make(map[string][]noderouter.UpdateOp)
	for _, ev := range events {
		if ev.Node == nil {
			continue
		}
		group, ok := r.nodeGroup(ev.Node)
		if !ok || group == "" {
			continue
		}
		id := ev.Node.GetNodeId()
		switch ev.Type {
		case cluster.NodeEventAdd:
			updates[group] = append(updates[group], noderouter.UpdateOp{Type: noderouter.UpdateAdd, IDs: []string{id}})
		case cluster.NodeEventDel:
			updates[group] = append(updates[group], noderouter.UpdateOp{Type: noderouter.UpdateRemove, IDs: []string{id}})
		}
	}
	if len(updates) == 0 {
		return
	}
	r.selector.Update(updates)
}

// PickActorNode 为未注册 Actor 选择部署节点.
func (r *Router) PickActorNode(uid gactor.ActorUID) (string, error) {
	nodeID, ok := r.actorFixedNode(uid)
	if ok {
		if nodeID == "" {
			return "", gactor.ErrNoAvailableNode
		}
		if !r.selector.Has(nodeID) {
			return "", fmt.Errorf("fixed actor node %s not available", nodeID)
		}
		return nodeID, nil
	}

	group, ok := r.actorNodeGroup(uid)
	if !ok || group == "" {
		return "", gactor.ErrNoAvailableNode
	}

	nodeIDs := r.selector.Pick(group, actorUIDRouteKey(uid), 1)
	if len(nodeIDs) == 0 || nodeIDs[0] == "" {
		return "", gactor.ErrNoAvailableNode
	}
	return nodeIDs[0], nil
}
