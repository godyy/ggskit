package redis

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	rand2 "math/rand"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// 分布式锁相关错误
var (
	// ErrLockNotObtained 锁获取失败
	ErrLockNotObtained = errors.New("lock not obtained")

	// ErrLockNotHeld 锁未被持有
	ErrLockNotHeld = errors.New("lock not held")
)

var (
	// 解锁脚本.
	unlockScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`)

	// 刷新脚本.
	refreshScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("PEXPIRE", KEYS[1], ARGV[2])
else
    return 0
end
`)
)

// DLockOpts 分布式锁配置选项
type DLockOpts struct {
	// Expiry 锁的过期时间，默认30秒
	Expiry time.Duration

	// RetryDelay 获取锁失败时的重试间隔，默认100毫秒
	RetryDelay time.Duration
}

// DefaultDLockOpts 返回默认配置
func DefaultDLockOpts() *DLockOpts {
	return &DLockOpts{
		Expiry:     30 * time.Second,
		RetryDelay: 100 * time.Millisecond,
	}
}

// DLock Redis分布式锁实现
type DLock struct {
	client Client
	key    string
	value  string
	opts   *DLockOpts
	locked bool
}

// NewDLock 创建一个新的分布式锁实例
// value 锁的值，用于标识锁的持有者，如果为空则自动生成UUID
func NewDLock(client Client, key, value string, opts *DLockOpts) *DLock {
	// 如果 value 为空, 自动生成.
	if value == "" {
		value = GenDLockValue()
	}

	// 生成默认选项.
	if opts == nil {
		opts = DefaultDLockOpts()
	}

	return &DLock{
		client: client,
		key:    key,
		value:  value,
		opts:   opts,
		locked: false,
	}
}

// Key 返回锁的键名
func (dl *DLock) Key() string {
	return dl.key
}

// IsLocked 检查锁是否被当前实例持有
func (dl *DLock) IsLocked() bool {
	return dl.locked
}

// Lock 获取锁，如果获取失败会阻塞直到获取成功或超时
func (dl *DLock) Lock(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := dl.TryLock(ctx); err == nil {
			return nil
		}

		// 等待一段时间后重试
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(dl.opts.RetryDelay):
			continue
		}
	}
}

// TryLock 尝试获取锁，不会阻塞
func (dl *DLock) TryLock(ctx context.Context) error {
	result := dl.client.SetNX(ctx, dl.key, dl.value, dl.opts.Expiry)
	if err := result.Err(); err != nil {
		return err
	}

	if result.Val() {
		dl.locked = true
		return nil
	}

	return ErrLockNotObtained
}

// Unlock 释放锁
func (dl *DLock) Unlock(ctx context.Context) error {
	if !dl.locked {
		return ErrLockNotHeld
	}

	result := unlockScript.Run(ctx, dl.client, []string{dl.key}, dl.value)
	if err := result.Err(); err != nil {
		return err
	}

	if result.Val().(int64) == 1 {
		dl.locked = false
		return nil
	}

	return ErrLockNotHeld
}

// Refresh 刷新锁的过期时间
func (dl *DLock) Refresh(ctx context.Context) error {
	if !dl.locked {
		return ErrLockNotHeld
	}

	expiry := dl.opts.Expiry.Milliseconds()
	result := refreshScript.Run(ctx, dl.client, []string{dl.key}, dl.value, expiry)
	if err := result.Err(); err != nil {
		return err
	}

	if result.Val().(int64) == 1 {
		return nil
	}

	return ErrLockNotHeld
}

// GenDLockValue 生成一个简单的UUID，用于标识锁的持有者.
func GenDLockValue() string {
	b := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		return fmt.Sprintf("%d", rand2.Int63())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant is 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
