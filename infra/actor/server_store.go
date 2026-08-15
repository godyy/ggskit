package actor

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	pkgerrors "github.com/pkg/errors"
	redis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

var (
	ErrActorServerIDInvalid = errors.New("server id must be greater than 0")
)

const serverStoreTimeout = 5 * time.Second

const (
	// serverStoreInvalidateChannel 用于广播 Actor 所属服务器缓存失效事件。
	serverStoreInvalidateChannel = "actor_server_store:invalidate"
)

// genActorServerKey 生成 Actor 所属服务器映射 key.
func genActorServerKey(uid ActorUID) string {
	return fmt.Sprintf("actor_server:%d:%d", uid.Category, uid.ID)
}

// ServerStoreConfig Actor 所属服务器存储配置.
type ServerStoreConfig struct {
	// Redis 客户端.
	RedisCli redis.UniversalClient

	// LocalCacheTTL 控制本地缓存和空值缓存的最长存活时间。
	// 默认值为 5 秒。
	LocalCacheTTL time.Duration
}

func (c *ServerStoreConfig) init() error {
	if c.RedisCli == nil {
		return errors.New("redis client is nil")
	}
	if c.LocalCacheTTL <= 0 {
		c.LocalCacheTTL = 5 * time.Second
	}
	return nil
}

// ServerStore Actor 所属服务器存储.
type ServerStore struct {
	cfg     *ServerStoreConfig // 配置
	sf      singleflight.Group // 用于并发控制
	cache   sync.Map           // 本地缓存
	invalPS *redis.PubSub      // 用于订阅失效事件
}

// serverStoreCacheEntry 表示一条本地缓存的 Actor 所属服务器记录。
type serverStoreCacheEntry struct {
	serverID int64
	exists   bool
	expireAt time.Time
}

// NewServerStore 创建 Actor 所属服务器存储.
func NewServerStore(cfg *ServerStoreConfig) (*ServerStore, error) {
	if err := cfg.init(); err != nil {
		return nil, err
	}
	store := &ServerStore{cfg: cfg}
	store.startInvalidationSubscriber()
	return store, nil
}

// SetActorServer 设置 Actor 所属服务器.
func (s *ServerStore) SetActorServer(uid ActorUID, serverID int64) error {
	if err := validateUID(uid); err != nil {
		return pkgerrors.WithMessage(err, "set actor server")
	}
	if serverID <= 0 {
		return pkgerrors.WithMessagef(ErrActorServerIDInvalid, "set actor server: invalid server id %d", serverID)
	}

	serverKey := genActorServerKey(uid)
	ctx, cancel := newServerStoreContext()
	defer cancel()

	if err := s.cfg.RedisCli.Set(ctx, serverKey, strconv.FormatInt(serverID, 10), 0).Err(); err != nil {
		return pkgerrors.WithMessage(err, "set actor server")
	}
	s.cacheServer(uid, serverID, true)
	s.publishInvalidation(uid)
	return nil
}

// GetActorServer 获取 Actor 所属服务器.
func (s *ServerStore) GetActorServer(uid ActorUID) (int64, bool, error) {
	if err := validateUID(uid); err != nil {
		return 0, false, pkgerrors.WithMessage(err, "get actor server")
	}

	if serverID, exists, ok := s.getCachedServer(uid); ok {
		return serverID, exists, nil
	}

	serverKey := genActorServerKey(uid)
	result, err, _ := s.sf.Do(serverKey, func() (any, error) {
		// 双检，避免等待期间其他请求已经回填缓存。
		if serverID, exists, ok := s.getCachedServer(uid); ok {
			return serverStoreCacheEntry{serverID: serverID, exists: exists}, nil
		}

		entry, err := s.getServerFromRedis(serverKey)
		if err != nil {
			return nil, err
		}
		s.cacheServer(uid, entry.serverID, entry.exists)
		return entry, nil
	})
	if err != nil {
		return 0, false, err
	}

	entry, ok := result.(serverStoreCacheEntry)
	if !ok {
		return 0, false, fmt.Errorf("unexpected actor server result type %T", result)
	}
	return entry.serverID, entry.exists, nil
}

// getServerFromRedis 从 Redis 读取 Actor 所属服务器。
func (s *ServerStore) getServerFromRedis(serverKey string) (serverStoreCacheEntry, error) {
	ctx, cancel := newServerStoreContext()
	defer cancel()

	raw, err := s.cfg.RedisCli.Get(ctx, serverKey).Result()
	if errors.Is(err, redis.Nil) {
		return serverStoreCacheEntry{exists: false}, nil
	}
	if err != nil {
		return serverStoreCacheEntry{}, pkgerrors.WithMessage(err, "get actor server")
	}

	serverID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return serverStoreCacheEntry{}, pkgerrors.WithMessagef(err, "parse actor server id %q", raw)
	}
	if serverID <= 0 {
		return serverStoreCacheEntry{}, pkgerrors.WithMessagef(ErrActorServerIDInvalid, "parse actor server id %q", raw)
	}

	return serverStoreCacheEntry{
		serverID: serverID,
		exists:   true,
	}, nil
}

// startInvalidationSubscriber 启动缓存失效事件订阅。
func (s *ServerStore) startInvalidationSubscriber() {
	pubsub := s.cfg.RedisCli.Subscribe(context.Background(), serverStoreInvalidateChannel)
	if _, err := pubsub.Receive(context.Background()); err != nil {
		_ = pubsub.Close()
		return
	}
	s.invalPS = pubsub

	go func() {
		ch := pubsub.Channel()
		for msg := range ch {
			if msg == nil || msg.Payload == "" {
				continue
			}
			uid, err := parseActorUIDPayload(msg.Payload)
			if err != nil {
				continue
			}
			s.deleteCachedServer(uid)
		}
	}()
}

// publishInvalidation 广播一条 Actor 所属服务器缓存失效事件。
func (s *ServerStore) publishInvalidation(uid ActorUID) {
	ctx, cancel := newServerStoreContext()
	defer cancel()

	_ = s.cfg.RedisCli.Publish(ctx, serverStoreInvalidateChannel, genActorUIDPayload(uid)).Err()
}

// getCachedServer 尝试读取本地缓存中的 Actor 所属服务器。
func (s *ServerStore) getCachedServer(uid ActorUID) (int64, bool, bool) {
	value, ok := s.cache.Load(uid)
	if !ok {
		return 0, false, false
	}

	entry, ok := value.(serverStoreCacheEntry)
	if !ok {
		s.cache.Delete(uid)
		return 0, false, false
	}
	if !entry.expireAt.After(time.Now()) {
		s.cache.Delete(uid)
		return 0, false, false
	}
	return entry.serverID, entry.exists, true
}

// cacheServer 将 Actor 所属服务器写入本地短缓存。
func (s *ServerStore) cacheServer(uid ActorUID, serverID int64, exists bool) {
	s.cache.Store(uid, serverStoreCacheEntry{
		serverID: serverID,
		exists:   exists,
		expireAt: time.Now().Add(s.cfg.LocalCacheTTL),
	})
}

// deleteCachedServer 删除本地缓存中的 Actor 所属服务器。
func (s *ServerStore) deleteCachedServer(uid ActorUID) {
	s.cache.Delete(uid)
}

func newServerStoreContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), serverStoreTimeout)
}
