package actor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godyy/gactor"
	redis "github.com/redis/go-redis/v9"
)

func newUncheckedServerStore(tb testing.TB) *ServerStore {
	tb.Helper()

	client := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:0",
	})
	tb.Cleanup(func() {
		_ = client.Close()
	})
	store, err := NewServerStore(&ServerStoreConfig{RedisCli: client})
	if err != nil {
		tb.Fatalf("new server store: %v", err)
	}
	return store
}

func setupServerStoreTestDriver(tb testing.TB) *ServerStore {
	tb.Helper()

	driver := setupRegistryTestDriver(tb)
	store, err := NewServerStore(&ServerStoreConfig{RedisCli: driver.cfg.RedisCli})
	if err != nil {
		tb.Fatalf("new server store: %v", err)
	}
	return store
}

func TestServerStoreSetAndGetActorServer(t *testing.T) {
	store := setupServerStoreTestDriver(t)
	uid := gactor.ActorUID{Category: 2, ID: 1001}

	if err := store.SetActorServer(uid, 101); err != nil {
		t.Fatalf("set actor server: %v", err)
	}

	serverID, ok, err := store.GetActorServer(uid)
	if err != nil {
		t.Fatalf("get actor server: %v", err)
	}
	if !ok {
		t.Fatal("expected actor server mapping to exist")
	}
	if serverID != 101 {
		t.Fatalf("unexpected actor server id: %d", serverID)
	}
}

func TestServerStoreGetActorServerNotExists(t *testing.T) {
	store := setupServerStoreTestDriver(t)

	serverID, ok, err := store.GetActorServer(gactor.ActorUID{Category: 2, ID: 1002})
	if err != nil {
		t.Fatalf("get actor server: %v", err)
	}
	if ok {
		t.Fatal("expected actor server mapping to be absent")
	}
	if serverID != 0 {
		t.Fatalf("expected zero actor server id, got %d", serverID)
	}
}

func TestServerStoreNilRedisClient(t *testing.T) {
	store, err := NewServerStore(&ServerStoreConfig{})
	if err == nil || err.Error() != "redis client is nil" {
		t.Fatalf("expected new store nil redis error, got %v", err)
	}
	if store != nil {
		t.Fatalf("expected nil store, got %#v", store)
	}
}

func TestServerStoreValidateParams(t *testing.T) {
	store := newUncheckedServerStore(t)

	tests := []struct {
		name    string
		call    func() error
		wantErr error
	}{
		{
			name: "set zero uid",
			call: func() error {
				return store.SetActorServer(gactor.ActorUID{}, 101)
			},
			wantErr: ErrActorUIDRequired,
		},
		{
			name: "set invalid server id",
			call: func() error {
				return store.SetActorServer(gactor.ActorUID{Category: 2, ID: 2201}, 0)
			},
			wantErr: ErrActorServerIDInvalid,
		},
		{
			name: "get zero uid",
			call: func() error {
				_, _, err := store.GetActorServer(gactor.ActorUID{})
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

func TestServerStoreGetActorServerInvalidValue(t *testing.T) {
	store := setupServerStoreTestDriver(t)
	uid := gactor.ActorUID{Category: 2, ID: 2301}

	ctx, cancel := context.WithTimeout(context.Background(), serverStoreTimeout)
	defer cancel()
	if err := store.cfg.RedisCli.Set(ctx, genActorServerKey(uid), "invalid", 0).Err(); err != nil {
		t.Fatalf("seed invalid actor server: %v", err)
	}

	_, _, err := store.GetActorServer(uid)
	if err == nil {
		t.Fatal("expected parse error for invalid actor server value")
	}
}

func TestServerStorePubSubInvalidatesLocalCache(t *testing.T) {
	storeA := setupServerStoreTestDriver(t)

	clientB := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   14,
	})
	t.Cleanup(func() {
		_ = clientB.Close()
	})

	storeB, err := NewServerStore(&ServerStoreConfig{RedisCli: clientB})
	if err != nil {
		t.Fatalf("new secondary server store: %v", err)
	}

	uid := gactor.ActorUID{Category: 2, ID: 2302}
	if setErr := storeA.SetActorServer(uid, 101); setErr != nil {
		t.Fatalf("set actor server: %v", setErr)
	}

	serverID, ok, err := storeB.GetActorServer(uid)
	if err != nil {
		t.Fatalf("get actor server: %v", err)
	}
	if !ok || serverID != 101 {
		t.Fatalf("unexpected actor server mapping: ok=%v serverID=%d", ok, serverID)
	}

	if updateErr := storeA.SetActorServer(uid, 202); updateErr != nil {
		t.Fatalf("update actor server: %v", updateErr)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		serverID, ok, err = storeB.GetActorServer(uid)
		if err != nil {
			t.Fatalf("get actor server after update: %v", err)
		}
		if ok && serverID == 202 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("expected local cache to be invalidated before fallback ttl elapsed, got ok=%v serverID=%d", ok, serverID)
}

func TestServerStoreInvalidatesNegativeCache(t *testing.T) {
	storeA := setupServerStoreTestDriver(t)

	clientB := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   14,
	})
	t.Cleanup(func() {
		_ = clientB.Close()
	})

	storeB, err := NewServerStore(&ServerStoreConfig{RedisCli: clientB})
	if err != nil {
		t.Fatalf("new secondary server store: %v", err)
	}

	uid := gactor.ActorUID{Category: 2, ID: 2303}
	serverID, ok, err := storeB.GetActorServer(uid)
	if err != nil {
		t.Fatalf("get missing actor server: %v", err)
	}
	if ok || serverID != 0 {
		t.Fatalf("expected missing actor server mapping, got ok=%v serverID=%d", ok, serverID)
	}

	if setErr := storeA.SetActorServer(uid, 303); setErr != nil {
		t.Fatalf("set actor server after negative lookup: %v", setErr)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		serverID, ok, err = storeB.GetActorServer(uid)
		if err != nil {
			t.Fatalf("get actor server after set: %v", err)
		}
		if ok && serverID == 303 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("expected negative cache to be invalidated before fallback ttl elapsed, got ok=%v serverID=%d", ok, serverID)
}
