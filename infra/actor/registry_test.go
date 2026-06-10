package actor

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godyy/gactor"
	redis "github.com/redis/go-redis/v9"
)

func newUncheckedRegistry(tb testing.TB) *Registry {
	tb.Helper()

	client := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:0",
	})
	tb.Cleanup(func() {
		_ = client.Close()
	})
	driver, err := NewRegistry(client)
	if err != nil {
		tb.Fatalf("new registry: %v", err)
	}
	return driver
}

func setupRegistryTestDriver(tb testing.TB) *Registry {
	tb.Helper()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   14,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		tb.Skipf("Redis not available: %v", err)
	}
	if err := client.FlushDB(ctx).Err(); err != nil {
		tb.Fatalf("flush db: %v", err)
	}

	tb.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})

	driver, err := NewRegistry(client)
	if err != nil {
		tb.Fatalf("new registry: %v", err)
	}
	return driver
}

func testActorUID(id int64) gactor.ActorUID {
	return gactor.ActorUID{
		Category: 1,
		ID:       id,
	}
}

func assertRemainingTTLInRange(t *testing.T, expireAt int64, minTTL int64, maxTTL int64) {
	t.Helper()
	if maxTTL <= 0 {
		if expireAt != 0 {
			t.Fatalf("expected expireAt 0, got %d", expireAt)
		}
		return
	}

	remainingTTL := expireAt - time.Now().Unix()
	if remainingTTL < minTTL || remainingTTL > maxTTL {
		t.Fatalf("remaining ttl out of range: got=%d want=[%d,%d] expireAt=%d", remainingTTL, minTTL, maxTTL, expireAt)
	}
}

func TestRegistryRegisterGetKeepAliveUnregister(t *testing.T) {
	driver := setupRegistryTestDriver(t)
	uid := testActorUID(1001)

	registerResult, err := driver.RegisterActor(gactor.ActorRegisterParams{
		UID:     uid,
		NodeId:  "node-a",
		LeaseId: "lease-a",
		TTL:     2,
	})
	if err != nil {
		t.Fatalf("register actor: %v", err)
	}
	if registerResult.NodeId != "node-a" {
		t.Fatalf("unexpected node id: %s", registerResult.NodeId)
	}
	assertRemainingTTLInRange(t, registerResult.ExpireAt, 1, 2)

	actorLoc, err := driver.GetActorLocation(uid)
	if err != nil {
		t.Fatalf("lookup actor registry: %v", err)
	}
	if actorLoc.NodeId != "node-a" {
		t.Fatalf("unexpected actor registry node id: %s", actorLoc.NodeId)
	}
	assertRemainingTTLInRange(t, actorLoc.ExpireAt, 1, 2)

	time.Sleep(1100 * time.Millisecond)

	if err = driver.KeepActorAlive(gactor.ActorKeepAliveParams{
		UID:     uid,
		NodeId:  "node-a",
		LeaseId: "lease-a",
		TTL:     3,
	}); err != nil {
		t.Fatalf("keep actor alive: %v", err)
	}

	actorLoc, err = driver.GetActorLocation(uid)
	if err != nil {
		t.Fatalf("lookup actor registry after keepalive: %v", err)
	}
	if actorLoc.NodeId != "node-a" {
		t.Fatalf("unexpected actor registry node id after keepalive: %s", actorLoc.NodeId)
	}
	assertRemainingTTLInRange(t, actorLoc.ExpireAt, 2, 3)

	if err = driver.UnregisterActor(gactor.ActorUnregisterParams{
		UID:     uid,
		NodeId:  "node-a",
		LeaseId: "lease-a",
	}); err != nil {
		t.Fatalf("unregister actor: %v", err)
	}

	_, err = driver.GetActorLocation(uid)
	if !errors.Is(err, gactor.ErrActorNotExists) {
		t.Fatalf("expected ErrActorNotExists after unregister, got %v", err)
	}
}

func TestRegistryRegisterConflict(t *testing.T) {
	driver := setupRegistryTestDriver(t)
	uid := testActorUID(1002)

	firstResult, err := driver.RegisterActor(gactor.ActorRegisterParams{
		UID:     uid,
		NodeId:  "node-a",
		LeaseId: "lease-a",
		TTL:     5,
	})
	if err != nil {
		t.Fatalf("first register actor: %v", err)
	}

	secondResult, err := driver.RegisterActor(gactor.ActorRegisterParams{
		UID:     uid,
		NodeId:  "node-b",
		LeaseId: "lease-b",
		TTL:     5,
	})
	if !errors.Is(err, gactor.ErrActorAlreadyRegistered) {
		t.Fatalf("expected ErrActorAlreadyRegistered, got %v", err)
	}
	if secondResult.NodeId != "node-a" {
		t.Fatalf("expected existing node-a, got %s", secondResult.NodeId)
	}
	if secondResult.ExpireAt < firstResult.ExpireAt-1 {
		t.Fatalf("expected expireAt to be preserved, first=%d second=%d", firstResult.ExpireAt, secondResult.ExpireAt)
	}
}

func TestRegistryLeaseMismatch(t *testing.T) {
	driver := setupRegistryTestDriver(t)
	uid := testActorUID(1003)

	if _, err := driver.RegisterActor(gactor.ActorRegisterParams{
		UID:     uid,
		NodeId:  "node-a",
		LeaseId: "lease-a",
		TTL:     5,
	}); err != nil {
		t.Fatalf("register actor: %v", err)
	}

	err := driver.KeepActorAlive(gactor.ActorKeepAliveParams{
		UID:     uid,
		NodeId:  "node-a",
		LeaseId: "lease-bad",
		TTL:     5,
	})
	if !errors.Is(err, gactor.ErrLeaseMismatch) {
		t.Fatalf("expected keepalive ErrLeaseMismatch, got %v", err)
	}

	err = driver.UnregisterActor(gactor.ActorUnregisterParams{
		UID:     uid,
		NodeId:  "node-a",
		LeaseId: "lease-bad",
	})
	if !errors.Is(err, gactor.ErrLeaseMismatch) {
		t.Fatalf("expected unregister ErrLeaseMismatch, got %v", err)
	}
}

func TestRegistryUnregisterActor(t *testing.T) {
	driver := setupRegistryTestDriver(t)
	uid := testActorUID(10031)

	if _, err := driver.RegisterActor(gactor.ActorRegisterParams{
		UID:     uid,
		NodeId:  "node-a",
		LeaseId: "lease-a",
		TTL:     5,
	}); err != nil {
		t.Fatalf("register actor: %v", err)
	}

	if err := driver.UnregisterActor(gactor.ActorUnregisterParams{
		UID:     uid,
		NodeId:  "node-a",
		LeaseId: "lease-a",
	}); err != nil {
		t.Fatalf("unregister actor: %v", err)
	}

	_, err := driver.GetActorLocation(uid)
	if !errors.Is(err, gactor.ErrActorNotExists) {
		t.Fatalf("expected ErrActorNotExists after unregister, got %v", err)
	}
}

func TestRegistryUnregisterActorNotExists(t *testing.T) {
	driver := setupRegistryTestDriver(t)

	err := driver.UnregisterActor(gactor.ActorUnregisterParams{
		UID:     testActorUID(10032),
		NodeId:  "node-a",
		LeaseId: "lease-a",
	})
	if !errors.Is(err, gactor.ErrActorNotExists) {
		t.Fatalf("expected ErrActorNotExists, got %v", err)
	}
}

func TestRegistryTTLExpiry(t *testing.T) {
	driver := setupRegistryTestDriver(t)
	uid := testActorUID(1004)

	if _, err := driver.RegisterActor(gactor.ActorRegisterParams{
		UID:     uid,
		NodeId:  "node-a",
		LeaseId: "lease-a",
		TTL:     1,
	}); err != nil {
		t.Fatalf("register actor: %v", err)
	}

	time.Sleep(1200 * time.Millisecond)

	_, err := driver.GetActorLocation(uid)
	if !errors.Is(err, gactor.ErrActorNotExists) {
		t.Fatalf("expected expired actor to disappear, got %v", err)
	}

	err = driver.KeepActorAlive(gactor.ActorKeepAliveParams{
		UID:     uid,
		NodeId:  "node-a",
		LeaseId: "lease-a",
		TTL:     1,
	})
	if !errors.Is(err, gactor.ErrActorNotExists) {
		t.Fatalf("expected keepalive on expired actor to fail with ErrActorNotExists, got %v", err)
	}
}

func TestRegistryConcurrentRegister(t *testing.T) {
	driver := setupRegistryTestDriver(t)
	uid := testActorUID(1005)

	type result struct {
		nodeId string
		err    error
	}

	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, nodeId := range []string{"node-a", "node-b"} {
		wg.Add(1)
		go func(nodeId string) {
			defer wg.Done()
			res, err := driver.RegisterActor(gactor.ActorRegisterParams{
				UID:     uid,
				NodeId:  nodeId,
				LeaseId: "lease-" + nodeId,
				TTL:     5,
			})
			results <- result{nodeId: res.NodeId, err: err}
		}(nodeId)
	}
	wg.Wait()
	close(results)

	var okCount int
	var conflictCount int
	var winnerNodeId string
	for res := range results {
		switch {
		case res.err == nil:
			okCount++
			winnerNodeId = res.nodeId
		case errors.Is(res.err, gactor.ErrActorAlreadyRegistered):
			conflictCount++
			if winnerNodeId == "" {
				winnerNodeId = res.nodeId
			} else if res.nodeId != winnerNodeId {
				t.Fatalf("conflict result returned different node: got=%s want=%s", res.nodeId, winnerNodeId)
			}
		default:
			t.Fatalf("unexpected register result: %v", res.err)
		}
	}

	if okCount != 1 || conflictCount != 1 {
		t.Fatalf("unexpected concurrent results: ok=%d conflict=%d", okCount, conflictCount)
	}

	actorLoc, err := driver.GetActorLocation(uid)
	if err != nil {
		t.Fatalf("lookup actor registry: %v", err)
	}
	if actorLoc.NodeId != winnerNodeId {
		t.Fatalf("stored winner mismatch: got=%s want=%s", actorLoc.NodeId, winnerNodeId)
	}
}

func TestRegistryNilRedisClient(t *testing.T) {
	driver, err := NewRegistry(nil)
	if err == nil || err.Error() != "redis client is nil" {
		t.Fatalf("expected new registry nil redis error, got %v", err)
	}
	if driver != nil {
		t.Fatalf("expected nil registry, got %#v", driver)
	}
}

func TestRegistryValidateParams(t *testing.T) {
	driver := newUncheckedRegistry(t)

	tests := []struct {
		name    string
		call    func() error
		wantErr error
	}{
		{
			name: "register zero uid",
			call: func() error {
				_, err := driver.RegisterActor(gactor.ActorRegisterParams{
					NodeId:  "node-a",
					LeaseId: "lease-a",
					TTL:     1,
				})
				return err
			},
			wantErr: ErrActorUIDRequired,
		},
		{
			name: "register empty node id",
			call: func() error {
				_, err := driver.RegisterActor(gactor.ActorRegisterParams{
					UID:     testActorUID(22001),
					LeaseId: "lease-a",
					TTL:     1,
				})
				return err
			},
			wantErr: ErrActorNodeIDRequired,
		},
		{
			name: "register empty lease id",
			call: func() error {
				_, err := driver.RegisterActor(gactor.ActorRegisterParams{
					UID:    testActorUID(22002),
					NodeId: "node-a",
					TTL:    1,
				})
				return err
			},
			wantErr: ErrActorLeaseIDRequired,
		},
		{
			name: "register negative ttl",
			call: func() error {
				_, err := driver.RegisterActor(gactor.ActorRegisterParams{
					UID:     testActorUID(22003),
					NodeId:  "node-a",
					LeaseId: "lease-a",
					TTL:     -1,
				})
				return err
			},
			wantErr: ErrActorTTLInvalid,
		},
		{
			name: "unregister zero uid",
			call: func() error {
				return driver.UnregisterActor(gactor.ActorUnregisterParams{
					NodeId:  "node-a",
					LeaseId: "lease-a",
				})
			},
			wantErr: ErrActorUIDRequired,
		},
		{
			name: "unregister empty node id",
			call: func() error {
				return driver.UnregisterActor(gactor.ActorUnregisterParams{
					UID:     testActorUID(22004),
					LeaseId: "lease-a",
				})
			},
			wantErr: ErrActorNodeIDRequired,
		},
		{
			name: "unregister empty lease id",
			call: func() error {
				return driver.UnregisterActor(gactor.ActorUnregisterParams{
					UID:    testActorUID(22005),
					NodeId: "node-a",
				})
			},
			wantErr: ErrActorLeaseIDRequired,
		},
		{
			name: "keepalive zero uid",
			call: func() error {
				return driver.KeepActorAlive(gactor.ActorKeepAliveParams{
					NodeId:  "node-a",
					LeaseId: "lease-a",
					TTL:     1,
				})
			},
			wantErr: ErrActorUIDRequired,
		},
		{
			name: "keepalive empty node id",
			call: func() error {
				return driver.KeepActorAlive(gactor.ActorKeepAliveParams{
					UID:     testActorUID(22006),
					LeaseId: "lease-a",
					TTL:     1,
				})
			},
			wantErr: ErrActorNodeIDRequired,
		},
		{
			name: "keepalive empty lease id",
			call: func() error {
				return driver.KeepActorAlive(gactor.ActorKeepAliveParams{
					UID:    testActorUID(22007),
					NodeId: "node-a",
					TTL:    1,
				})
			},
			wantErr: ErrActorLeaseIDRequired,
		},
		{
			name: "keepalive negative ttl",
			call: func() error {
				return driver.KeepActorAlive(gactor.ActorKeepAliveParams{
					UID:     testActorUID(22008),
					NodeId:  "node-a",
					LeaseId: "lease-a",
					TTL:     -1,
				})
			},
			wantErr: ErrActorTTLInvalid,
		},
		{
			name: "lookup zero uid",
			call: func() error {
				_, err := driver.GetActorLocation(gactor.ActorUID{})
				return err
			},
			wantErr: ErrActorUIDRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func BenchmarkRegistryRegisterActor(b *testing.B) {
	driver := setupRegistryTestDriver(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		uid := testActorUID(int64(i + 1))
		if _, err := driver.RegisterActor(gactor.ActorRegisterParams{
			UID:     uid,
			NodeId:  "node-bench",
			LeaseId: "lease-bench",
			TTL:     30,
		}); err != nil {
			b.Fatalf("register actor: %v", err)
		}
	}
}

func BenchmarkRegistryRegisterActorParallel(b *testing.B) {
	driver := setupRegistryTestDriver(b)
	var nextID atomic.Int64

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			uid := testActorUID(4000 + nextID.Add(1))
			if _, err := driver.RegisterActor(gactor.ActorRegisterParams{
				UID:     uid,
				NodeId:  "node-bench",
				LeaseId: "lease-bench",
				TTL:     30,
			}); err != nil {
				b.Fatalf("register actor: %v", err)
			}
		}
	})
}

func BenchmarkRegistryLookupActor(b *testing.B) {
	driver := setupRegistryTestDriver(b)
	uid := testActorUID(2001)
	if _, err := driver.RegisterActor(gactor.ActorRegisterParams{
		UID:     uid,
		NodeId:  "node-bench",
		LeaseId: "lease-bench",
		TTL:     30,
	}); err != nil {
		b.Fatalf("register actor: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := driver.GetActorLocation(uid); err != nil {
			b.Fatalf("lookup actor registry: %v", err)
		}
	}
}

func BenchmarkRegistryLookupActorParallel(b *testing.B) {
	driver := setupRegistryTestDriver(b)
	var nextID atomic.Int64

	for i := range 128 {
		uid := testActorUID(5000 + int64(i))
		if _, err := driver.RegisterActor(gactor.ActorRegisterParams{
			UID:     uid,
			NodeId:  "node-bench",
			LeaseId: "lease-bench",
			TTL:     30,
		}); err != nil {
			b.Fatalf("register actor: %v", err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			uid := testActorUID(5000 + nextID.Add(1)%128)
			if _, err := driver.GetActorLocation(uid); err != nil {
				b.Fatalf("lookup actor registry: %v", err)
			}
		}
	})
}

func BenchmarkRegistryKeepActorAlive(b *testing.B) {
	driver := setupRegistryTestDriver(b)
	uid := testActorUID(2002)
	if _, err := driver.RegisterActor(gactor.ActorRegisterParams{
		UID:     uid,
		NodeId:  "node-bench",
		LeaseId: "lease-bench",
		TTL:     30,
	}); err != nil {
		b.Fatalf("register actor: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := driver.KeepActorAlive(gactor.ActorKeepAliveParams{
			UID:     uid,
			NodeId:  "node-bench",
			LeaseId: "lease-bench",
			TTL:     30,
		}); err != nil {
			b.Fatalf("keep actor alive: %v", err)
		}
	}
}

func BenchmarkRegistryKeepActorAliveParallel(b *testing.B) {
	driver := setupRegistryTestDriver(b)
	var nextID atomic.Int64

	for i := range 128 {
		uid := testActorUID(6000 + int64(i))
		if _, err := driver.RegisterActor(gactor.ActorRegisterParams{
			UID:     uid,
			NodeId:  "node-bench",
			LeaseId: "lease-bench",
			TTL:     30,
		}); err != nil {
			b.Fatalf("register actor: %v", err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			uid := testActorUID(6000 + nextID.Add(1)%128)
			if err := driver.KeepActorAlive(gactor.ActorKeepAliveParams{
				UID:     uid,
				NodeId:  "node-bench",
				LeaseId: "lease-bench",
				TTL:     30,
			}); err != nil {
				b.Fatalf("keep actor alive: %v", err)
			}
		}
	})
}

func BenchmarkRegistryUnregisterActor(b *testing.B) {
	driver := setupRegistryTestDriver(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		uid := testActorUID(int64(3000 + i))
		if _, err := driver.RegisterActor(gactor.ActorRegisterParams{
			UID:     uid,
			NodeId:  "node-bench",
			LeaseId: "lease-bench",
			TTL:     30,
		}); err != nil {
			b.Fatalf("register actor: %v", err)
		}
		if err := driver.UnregisterActor(gactor.ActorUnregisterParams{
			UID:     uid,
			NodeId:  "node-bench",
			LeaseId: "lease-bench",
		}); err != nil {
			b.Fatalf("unregister actor: %v", err)
		}
	}
}

func BenchmarkRegistryUnregisterActorParallel(b *testing.B) {
	driver := setupRegistryTestDriver(b)
	var nextID atomic.Int64

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id := 7000 + nextID.Add(1)
			uid := testActorUID(id)
			leaseID := "lease-bench-" + strconv.FormatInt(id, 10)
			if _, err := driver.RegisterActor(gactor.ActorRegisterParams{
				UID:     uid,
				NodeId:  "node-bench",
				LeaseId: leaseID,
				TTL:     30,
			}); err != nil {
				b.Fatalf("register actor: %v", err)
			}
			if err := driver.UnregisterActor(gactor.ActorUnregisterParams{
				UID:     uid,
				NodeId:  "node-bench",
				LeaseId: leaseID,
			}); err != nil {
				b.Fatalf("unregister actor: %v", err)
			}
		}
	})
}
