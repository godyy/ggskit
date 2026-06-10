package redis

import (
	"context"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// 测试用的Redis客户端
func setupTestClient(t *testing.T) goredis.UniversalClient {
	client := goredis.NewClient(&goredis.Options{
		Addr: "localhost:6379",
		DB:   15, // 使用测试数据库
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	// 清理测试数据库
	client.FlushDB(ctx)

	return client
}

func TestNew(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	// 测试默认配置
	lock := NewDLock(client, "test-key", "", nil)
	if lock == nil {
		t.Error("Expected lock to be created")
	}
	if lock.Key() != "test-key" {
		t.Errorf("Expected key to be 'test-key', got %s", lock.Key())
	}
	if lock.IsLocked() {
		t.Error("Expected lock to not be locked initially")
	}

	// 测试自定义配置
	opts := &DLockOpts{
		Expiry:     10 * time.Second,
		RetryDelay: 50 * time.Millisecond,
	}
	lock2 := NewDLock(client, "test-key-2", "test-value", opts)
	if lock2 == nil {
		t.Error("Expected lock2 to be created")
	}
	if lock2.Key() != "test-key-2" {
		t.Errorf("Expected key to be 'test-key-2', got %s", lock2.Key())
	}
}

func TestTryLock(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()
	lock := NewDLock(client, "test-lock", "", nil)

	// 第一次获取锁应该成功
	err := lock.TryLock(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !lock.IsLocked() {
		t.Error("Expected lock to be locked")
	}

	// 再次尝试获取同一个锁应该失败
	lock2 := NewDLock(client, "test-lock", "", nil)
	err = lock2.TryLock(ctx)
	if err != ErrLockNotObtained {
		t.Errorf("Expected ErrLockNotObtained, got %v", err)
	}
	if lock2.IsLocked() {
		t.Error("Expected lock2 to not be locked")
	}

	// 释放锁
	err = lock.Unlock(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if lock.IsLocked() {
		t.Error("Expected lock to not be locked after unlock")
	}

	// 释放后应该能再次获取
	err = lock2.TryLock(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !lock2.IsLocked() {
		t.Error("Expected lock2 to be locked")
	}

	lock2.Unlock(ctx)
}

func TestLock(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lock := NewDLock(client, "test-lock-blocking", "", &DLockOpts{
		Expiry:     2 * time.Second,
		RetryDelay: 100 * time.Millisecond,
	})

	// 获取锁
	err := lock.Lock(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !lock.IsLocked() {
		t.Error("Expected lock to be locked")
	}

	// 在另一个goroutine中尝试获取同一个锁
	done := make(chan bool)
	go func() {
		lock2 := NewDLock(client, "test-lock-blocking", "", &DLockOpts{
			RetryDelay: 50 * time.Millisecond,
		})
		ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel2()
		err := lock2.Lock(ctx2)
		// 应该超时
		if err == nil {
			t.Error("Expected error due to timeout")
		}
		done <- true
	}()

	<-done
	lock.Unlock(ctx)
}

func TestUnlock(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()
	lock := NewDLock(client, "test-unlock", "", nil)

	// 未获取锁时释放应该失败
	err := lock.Unlock(ctx)
	if err != ErrLockNotHeld {
		t.Errorf("Expected ErrLockNotHeld, got %v", err)
	}

	// 获取锁后释放应该成功
	err = lock.TryLock(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	err = lock.Unlock(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if lock.IsLocked() {
		t.Error("Expected lock to not be locked after unlock")
	}

	// 重复释放应该失败
	err = lock.Unlock(ctx)
	if err != ErrLockNotHeld {
		t.Errorf("Expected ErrLockNotHeld, got %v", err)
	}
}

func TestRefresh(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()
	lock := NewDLock(client, "test-refresh", "", &DLockOpts{
		Expiry: 1 * time.Second,
	})

	// 未获取锁时刷新应该失败
	err := lock.Refresh(ctx)
	if err != ErrLockNotHeld {
		t.Errorf("Expected ErrLockNotHeld, got %v", err)
	}

	// 获取锁后刷新应该成功
	err = lock.TryLock(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	err = lock.Refresh(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	lock.Unlock(ctx)
}

func TestLockExpiry(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()
	lock := NewDLock(client, "test-expiry", "", &DLockOpts{
		Expiry: 500 * time.Millisecond,
	})

	// 获取锁
	err := lock.TryLock(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// 等待锁过期
	time.Sleep(600 * time.Millisecond)

	// 另一个实例应该能获取锁
	lock2 := NewDLock(client, "test-expiry", "", nil)
	err = lock2.TryLock(ctx)
	if err != nil {
		t.Errorf("Expected no error after expiry, got %v", err)
	}

	lock2.Unlock(ctx)
}

func TestConcurrentLocks(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	const numWorkers = 10
	const lockKey = "test-concurrent"

	successCount := make(chan int, numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			lock := NewDLock(client, lockKey, "", &DLockOpts{
				Expiry:     1 * time.Second,
				RetryDelay: 10 * time.Millisecond,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			if err := lock.Lock(ctx); err == nil {
				successCount <- workerID
				time.Sleep(100 * time.Millisecond) // 持有锁一段时间
				lock.Unlock(ctx)
			}
		}(i)
	}

	// 应该至少有一个worker成功获取锁
	select {
	case <-successCount:
		// 成功
	case <-time.After(5 * time.Second):
		t.Error("Expected at least one worker to acquire lock")
	}
}
