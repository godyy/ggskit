package actor

import (
	"errors"
	"fmt"
	"testing"

	"github.com/godyy/gactor"
	"github.com/godyy/ggskit/infra/cluster"
)

func mustNewRouter(t *testing.T, cfg RouterConfig) *Router {
	t.Helper()

	router, err := NewRouter(cfg)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	return router
}

func TestRouterPickActorNode(t *testing.T) {
	router := mustNewRouter(t, RouterConfig{
		NodeGroup: func(node *cluster.Node) (string, bool) {
			if node == nil {
				return "", false
			}
			return node.Category, true
		},
		ActorNodeGroup: func(uid gactor.ActorUID) (string, bool) {
			switch uid.Category {
			case 1:
				return "game", true
			case 2:
				return "agent", true
			default:
				return "", false
			}
		},
		ActorFixedNode: func(uid gactor.ActorUID) (string, bool) {
			return "", false
		},
	})
	router.SetNodes([]*cluster.Node{
		cluster.NewNode("game", "a", ""),
		cluster.NewNode("game", "b", ""),
		cluster.NewNode("agent", "a", ""),
	}, true)

	nodeID, err := router.PickActorNode(gactor.ActorUID{
		Category: 1,
		ID:       1001,
	})
	if err != nil {
		t.Fatalf("pick actor node: %v", err)
	}
	if nodeID != "game/a" && nodeID != "game/b" {
		t.Fatalf("unexpected node id: %s", nodeID)
	}
}

func TestRouterPickActorNodeNotExists(t *testing.T) {
	router := mustNewRouter(t, RouterConfig{
		NodeGroup: func(node *cluster.Node) (string, bool) {
			return node.Category, true
		},
		ActorFixedNode: func(uid gactor.ActorUID) (string, bool) {
			return "", false
		},
		ActorNodeGroup: func(uid gactor.ActorUID) (string, bool) {
			return "game", true
		},
	})

	_, err := router.PickActorNode(gactor.ActorUID{
		Category: 1,
		ID:       1002,
	})
	if !errors.Is(err, gactor.ErrActorNotExists) {
		t.Fatalf("expected ErrActorNotExists, got %v", err)
	}
}

func TestRouterPickActorNodeDoesNotUseRegisteredLocation(t *testing.T) {
	driver := setupRegistryTestDriver(t)
	router := mustNewRouter(t, RouterConfig{
		NodeGroup: func(node *cluster.Node) (string, bool) {
			return node.Category, true
		},
		ActorFixedNode: func(uid gactor.ActorUID) (string, bool) {
			return "", false
		},
		ActorNodeGroup: func(uid gactor.ActorUID) (string, bool) {
			return "game", true
		},
	})
	router.SetNodes([]*cluster.Node{
		cluster.NewNode("game", "router-node", ""),
	}, true)

	uid := testActorUID(1003)
	if _, err := driver.RegisterActor(gactor.ActorRegisterParams{
		UID:     uid,
		NodeId:  "registered-node",
		LeaseId: "lease-a",
		TTL:     30,
	}); err != nil {
		t.Fatalf("register actor: %v", err)
	}

	nodeID, err := router.PickActorNode(uid)
	if err != nil {
		t.Fatalf("pick actor node: %v", err)
	}
	if nodeID != "game/router-node" {
		t.Fatalf("expected game/router-node, got %s", nodeID)
	}
}

func TestRouterPickActorNodeByActorCategory(t *testing.T) {
	router := mustNewRouter(t, RouterConfig{
		NodeGroup: func(node *cluster.Node) (string, bool) {
			if node == nil {
				return "", false
			}
			return node.Category, true
		},
		ActorNodeGroup: func(uid gactor.ActorUID) (string, bool) {
			switch uid.Category {
			case 1:
				return "game", true
			case 2:
				return "agent", true
			default:
				return "", false
			}
		},
		ActorFixedNode: func(uid gactor.ActorUID) (string, bool) {
			return "", false
		},
	})
	router.SetNodes([]*cluster.Node{
		cluster.NewNode("game", "node-a", ""),
		cluster.NewNode("agent", "node-b", ""),
	}, true)

	nodeID, err := router.PickActorNode(gactor.ActorUID{Category: 2, ID: 1004})
	if err != nil {
		t.Fatalf("pick actor node: %v", err)
	}
	if nodeID != "agent/node-b" {
		t.Fatalf("expected agent/node-b, got %s", nodeID)
	}
}

func TestRouterUsesCustomNodeGroupStrategy(t *testing.T) {
	router := mustNewRouter(t, RouterConfig{
		NodeGroup: func(node *cluster.Node) (string, bool) {
			if node == nil {
				return "", false
			}
			return fmt.Sprintf("%s/%d", node.Category, node.ServerId), true
		},
		ActorNodeGroup: func(uid gactor.ActorUID) (string, bool) {
			if uid.Category != 1 {
				return "", false
			}
			return "game/1", true
		},
		ActorFixedNode: func(uid gactor.ActorUID) (string, bool) {
			return "", false
		},
	})
	router.SetNodes([]*cluster.Node{
		{Category: "game", Name: "node-a", ServerId: 1},
		{Category: "game", Name: "node-b", ServerId: 2},
	}, true)

	nodeID, err := router.PickActorNode(gactor.ActorUID{Category: 1, ID: 1005})
	if err != nil {
		t.Fatalf("pick actor node: %v", err)
	}
	if nodeID != "game/node-a" {
		t.Fatalf("expected game/node-a, got %s", nodeID)
	}
}

func TestRouterPickActorNodeFixedNode(t *testing.T) {
	router := mustNewRouter(t, RouterConfig{
		NodeGroup: func(node *cluster.Node) (string, bool) {
			if node == nil {
				return "", false
			}
			return node.Category, true
		},
		ActorFixedNode: func(uid gactor.ActorUID) (string, bool) {
			if uid.Category != 1 {
				return "", false
			}
			return "game/fixed-node", true
		},
		ActorNodeGroup: func(uid gactor.ActorUID) (string, bool) {
			return "game", true
		},
	})
	router.SetNodes([]*cluster.Node{
		cluster.NewNode("game", "fixed-node", ""),
		cluster.NewNode("game", "other-node", ""),
	}, true)

	nodeID, err := router.PickActorNode(gactor.ActorUID{Category: 1, ID: 1006})
	if err != nil {
		t.Fatalf("pick actor node: %v", err)
	}
	if nodeID != "game/fixed-node" {
		t.Fatalf("expected game/fixed-node, got %s", nodeID)
	}
}

func TestRouterPickActorNodeFixedNodeUnavailable(t *testing.T) {
	router := mustNewRouter(t, RouterConfig{
		NodeGroup: func(node *cluster.Node) (string, bool) {
			if node == nil {
				return "", false
			}
			return node.Category, true
		},
		ActorFixedNode: func(uid gactor.ActorUID) (string, bool) {
			if uid.Category != 1 {
				return "", false
			}
			return "game/fixed-node", true
		},
		ActorNodeGroup: func(uid gactor.ActorUID) (string, bool) {
			return "game", true
		},
	})
	router.SetNodes([]*cluster.Node{
		cluster.NewNode("game", "other-node", ""),
	}, true)

	_, err := router.PickActorNode(gactor.ActorUID{Category: 1, ID: 1007})
	if err == nil {
		t.Fatal("expected fixed node unavailable error")
	}
	if !errors.Is(err, gactor.ErrActorNotExists) && err.Error() != "fixed actor node game/fixed-node not available" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRouterValidateRequiredFuncs(t *testing.T) {
	tests := []struct {
		name string
		cfg  RouterConfig
		want string
	}{
		{
			name: "missing node group",
			cfg: RouterConfig{
				ActorFixedNode: func(uid gactor.ActorUID) (string, bool) { return "", false },
				ActorNodeGroup: func(uid gactor.ActorUID) (string, bool) { return "", false },
			},
			want: "router node group func is required",
		},
		{
			name: "missing actor fixed node",
			cfg: RouterConfig{
				NodeGroup:      func(node *cluster.Node) (string, bool) { return "", false },
				ActorNodeGroup: func(uid gactor.ActorUID) (string, bool) { return "", false },
			},
			want: "router actor fixed node func is required",
		},
		{
			name: "missing actor node group",
			cfg: RouterConfig{
				NodeGroup:      func(node *cluster.Node) (string, bool) { return "", false },
				ActorFixedNode: func(uid gactor.ActorUID) (string, bool) { return "", false },
			},
			want: "router actor node group func is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, err := NewRouter(tt.cfg)
			if err == nil {
				t.Fatal("expected constructor error")
			}
			if err.Error() != tt.want {
				t.Fatalf("unexpected error: %v", err)
			}
			if router != nil {
				t.Fatal("expected nil router")
			}
		})
	}
}
