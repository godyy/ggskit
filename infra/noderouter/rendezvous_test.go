package noderouter

import (
	"fmt"
	"slices"
	"sync"
	"testing"
)

func TestRendezvousSelector_Basic(t *testing.T) {
	s := NewRendezvousSelector()
	nodes := []string{"node1", "node2", "node3"}
	group := "test-group"

	// Test Set（增量设置该组）
	s.Set(map[string][]string{group: nodes}, false)

	// Test Pick（n=1 选单个候选）
	key := []byte("test-key")
	cand := s.Pick(group, key, 1)
	if cand == nil || len(cand) != 1 {
		t.Fatal("expected one candidate")
	}
	node := cand[0]
	if !contains(nodes, node) {
		t.Errorf("expected node to be in %v, got %s", nodes, node)
	}

	// Test Consistency（一致性）
	cand2 := s.Pick(group, key, 1)
	if cand2 == nil || cand2[0] != node {
		t.Errorf("expected consistent result %s, got %v", node, cand2)
	}

	// Test PickN（前 n 个候选）
	candidates := s.Pick(group, key, 2)
	if len(candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(candidates))
	}
	for _, c := range candidates {
		if !contains(nodes, c) {
			t.Errorf("expected candidate %s to be in nodes", c)
		}
	}
	if candidates[0] == candidates[1] {
		t.Error("expected distinct candidates")
	}
}

func TestRendezvousSelector_AddRemove(t *testing.T) {
	s := NewRendezvousSelector()
	group := "dynamic-group"

	// Start empty
	s.Set(map[string][]string{group: {}}, false)
	cand := s.Pick(group, []byte("key"), 1)
	if cand != nil {
		t.Error("expected empty for empty group")
	}

	// Add nodes（增量添加）
	s.Update(map[string][]UpdateOp{group: {{Type: UpdateAdd, IDs: []string{"node1"}}}})
	cand = s.Pick(group, []byte("key"), 1)
	if cand == nil || cand[0] != "node1" {
		t.Errorf("expected node1, got %v", cand)
	}

	s.Update(map[string][]UpdateOp{group: {{Type: UpdateAdd, IDs: []string{"node2"}}}})
	cand = s.Pick(group, []byte("key"), 1)
	if cand == nil || !contains([]string{"node1", "node2"}, cand[0]) {
		t.Errorf("expected node1 or node2, got %v", cand)
	}

	// Remove node（增量移除）
	s.Update(map[string][]UpdateOp{group: {{Type: UpdateRemove, IDs: []string{"node1"}}}})
	cand = s.Pick(group, []byte("key"), 1)
	if cand == nil || cand[0] != "node2" {
		t.Errorf("expected node2, got %v", cand)
	}

	// Remove remaining（移除剩余）
	s.Update(map[string][]UpdateOp{group: {{Type: UpdateRemove, IDs: []string{"node2"}}}})
	cand = s.Pick(group, []byte("key"), 1)
	if cand != nil {
		t.Error("expected empty after removing all nodes")
	}
}

func TestRendezvousSelector_RemoveGroup_ByFullReplace(t *testing.T) {
	s := NewRendezvousSelector()
	group := "del-group"
	// 先设置该组
	s.Set(map[string][]string{group: {"n1"}}, false)
	if cand := s.Pick(group, []byte("k"), 1); cand == nil {
		t.Fatal("setup failed")
	}
	// 使用全量替换清空所有组
	s.Set(map[string][]string{}, true)
	if cand := s.Pick(group, []byte("k"), 1); cand != nil {
		t.Error("expected empty after full replace clearing groups")
	}
}

func TestRendezvousSelector_UnknownGroup(t *testing.T) {
	s := NewRendezvousSelector()
	cand := s.Pick("unknown", []byte("k"), 1)
	if cand != nil {
		t.Error("expected nil for unknown group (n=1)")
	}
	cand = s.Pick("unknown", []byte("k"), 2)
	if cand != nil {
		t.Error("expected nil for unknown group (n=2)")
	}
}

func TestRendezvousSelector_Has(t *testing.T) {
	s := NewRendezvousSelector()

	s.Set(map[string][]string{
		"group-a": {"n1", "n2"},
		"group-b": {"n3"},
	}, true)
	if !s.Has("n1") || !s.Has("n3") {
		t.Fatal("expected existing nodes to be found")
	}
	if s.Has("n4") {
		t.Fatal("expected unknown node to be absent")
	}

	s.Set(map[string][]string{
		"group-a": {"n2"},
	}, false)
	if s.Has("n1") {
		t.Fatal("expected replaced group node n1 to be removed")
	}
	if !s.Has("n2") || !s.Has("n3") {
		t.Fatal("expected retained nodes to still exist")
	}

	s.Update(map[string][]UpdateOp{
		"group-b": {
			{Type: UpdateRemove, IDs: []string{"n3"}},
			{Type: UpdateAdd, IDs: []string{"n4"}},
		},
	})
	if s.Has("n3") {
		t.Fatal("expected removed node n3 to be absent")
	}
	if !s.Has("n4") {
		t.Fatal("expected added node n4 to exist")
	}
}

func TestRendezvousSelector_Concurrency(t *testing.T) {
	s := NewRendezvousSelector()
	group := "concurrent-group"
	s.Set(map[string][]string{group: {"n1", "n2", "n3"}}, false)

	var wg sync.WaitGroup
	// Writers（并发增删）
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			nodeID := fmt.Sprintf("n%d", id)
			if id%2 == 0 {
				s.Update(map[string][]UpdateOp{group: {{Type: UpdateAdd, IDs: []string{nodeID}}}})
			} else {
				s.Update(map[string][]UpdateOp{group: {{Type: UpdateRemove, IDs: []string{nodeID}}}})
			}
		}(i)
	}

	// Readers（并发读取）
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Pick(group, []byte("key"), 1)
		}()
	}

	wg.Wait()
}

func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}
