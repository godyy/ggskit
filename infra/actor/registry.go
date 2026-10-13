package actor

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/godyy/gactor"
	pkgerrors "github.com/pkg/errors"
	redis "github.com/redis/go-redis/v9"
	"github.com/rs/xid"
	"golang.org/x/sync/singleflight"
)

const (
	// registryTimeout 注册表操作超时时间。
	registryTimeout = 5 * time.Second

	// registryChangeStream 保存可回放的 Actor Location 变更事件。
	registryChangeStream = "actor_registry:changes"

	// registryStreamReadBlock 控制 Stream 空闲时的最长阻塞读取时间。
	registryStreamReadBlock = 100 * time.Millisecond
)

const (
	registryScriptResultOK = iota
	registryScriptResultNotExists
	registryScriptResultAlreadyRegistered
	registryScriptResultLeaseMismatch
)

var (
	ErrActorUIDRequired           = errors.New("actor uid is required")
	ErrActorNodeIDRequired        = errors.New("node id is required")
	ErrActorLeaseIDRequired       = errors.New("lease id is required")
	ErrActorSourceLeaseIDRequired = errors.New("source lease id is required")
	ErrActorTTLInvalid            = errors.New("ttl must be greater than or equal to 0")

	registerActorScript = redis.NewScript(`
local keyType = redis.call("TYPE", KEYS[1])["ok"]
local currentNodeId = ""
local currentTTL = 0

if keyType == "hash" then
    currentNodeId = redis.call("HGET", KEYS[1], "node_id") or ""
elseif keyType == "string" then
    local raw = redis.call("GET", KEYS[1])
    if raw then
        local current = cjson.decode(raw)
        currentNodeId = current["NodeId"] or ""
    end
end

if currentNodeId ~= "" then
    currentTTL = redis.call("PTTL", KEYS[1])
    if currentTTL > 0 then
        currentTTL = math.floor((currentTTL + 999) / 1000)
    end

    if currentNodeId ~= ARGV[1] then
        return {2, currentNodeId, currentTTL}
    end
end

if keyType == "string" then
    redis.call("DEL", KEYS[1])
end
redis.call("HSET", KEYS[1],
    "node_id", ARGV[1],
    "lease_id", ARGV[2]
)

local ttl = tonumber(ARGV[3]) or 0
if ttl > 0 then
    redis.call("EXPIRE", KEYS[1], ttl)
    return {0, ARGV[1], ttl}
end

redis.call("PERSIST", KEYS[1])
return {0, ARGV[1], 0}
`)

	takeOverActorScript = redis.NewScript(`
local keyType = redis.call("TYPE", KEYS[1])["ok"]
if keyType == "none" then
    return {1}
end

local currentLeaseId = ""
if keyType == "hash" then
    currentLeaseId = redis.call("HGET", KEYS[1], "lease_id") or ""
elseif keyType == "string" then
    local raw = redis.call("GET", KEYS[1])
    if not raw then
        return {1}
    end

    local reg = cjson.decode(raw)
    currentLeaseId = reg["LeaseId"] or ""
end

if currentLeaseId ~= ARGV[3] then
    return {3}
end

if keyType == "string" then
    redis.call("DEL", KEYS[1])
end
redis.call("HSET", KEYS[1],
    "node_id", ARGV[1],
    "lease_id", ARGV[2]
)
local revision = redis.call("HINCRBY", KEYS[1], "revision", 1)
redis.call("XADD", KEYS[2], "MAXLEN", "~", ARGV[5], "*",
    "category", ARGV[6],
    "id", ARGV[7],
    "revision", revision,
    "op", "upsert"
)

local ttl = tonumber(ARGV[4]) or 0
if ttl > 0 then
    redis.call("EXPIRE", KEYS[1], ttl)
    return {0, ARGV[1], ttl}
end

redis.call("PERSIST", KEYS[1])
return {0, ARGV[1], 0}
`)

	unregisterActorScript = redis.NewScript(`
local keyType = redis.call("TYPE", KEYS[1])["ok"]
if keyType == "none" then
    return {1}
end

local currentNodeId = ""
local currentLeaseId = ""

if keyType == "hash" then
    local values = redis.call("HMGET", KEYS[1], "node_id", "lease_id")
    currentNodeId = values[1] or ""
    currentLeaseId = values[2] or ""
elseif keyType == "string" then
    local raw = redis.call("GET", KEYS[1])
    if not raw then
        return {1}
    end

    local reg = cjson.decode(raw)
    currentNodeId = reg["NodeId"] or ""
    currentLeaseId = reg["LeaseId"] or ""
end

if currentNodeId ~= ARGV[1] or currentLeaseId ~= ARGV[2] then
    return {3}
end

local revision = 1
if keyType == "hash" then
    revision = tonumber(redis.call("HGET", KEYS[1], "revision") or "0") + 1
end
redis.call("XADD", KEYS[2], "MAXLEN", "~", ARGV[3], "*",
    "category", ARGV[4],
    "id", ARGV[5],
    "revision", revision,
    "op", "delete"
)
redis.call("DEL", KEYS[1])
return {0}
`)

	keepAliveActorScript = redis.NewScript(`
local keyType = redis.call("TYPE", KEYS[1])["ok"]
if keyType == "none" then
    return {1}
end

local currentNodeId = ""
local currentLeaseId = ""

if keyType == "hash" then
    local values = redis.call("HMGET", KEYS[1], "node_id", "lease_id")
    currentNodeId = values[1] or ""
    currentLeaseId = values[2] or ""
elseif keyType == "string" then
    local raw = redis.call("GET", KEYS[1])
    if not raw then
        return {1}
    end

    local reg = cjson.decode(raw)
    currentNodeId = reg["NodeId"] or ""
    currentLeaseId = reg["LeaseId"] or ""
end

if currentNodeId ~= ARGV[1] or currentLeaseId ~= ARGV[2] then
    return {3}
end

local ttl = tonumber(ARGV[3]) or 0
if ttl > 0 then
    redis.call("EXPIRE", KEYS[1], ttl)
else
    redis.call("PERSIST", KEYS[1])
end

return {0}
`)

	getActorRegScript = redis.NewScript(`
local keyType = redis.call("TYPE", KEYS[1])["ok"]
local nodeId = ""

if keyType == "hash" then
    nodeId = redis.call("HGET", KEYS[1], "node_id") or ""
elseif keyType == "string" then
    local raw = redis.call("GET", KEYS[1])
    if raw then
        local reg = cjson.decode(raw)
        nodeId = reg["NodeId"] or ""
    end
end

if nodeId == "" then
    return {1}
end

local ttlSeconds = 0
local ttl = redis.call("PTTL", KEYS[1])
if ttl > 0 then
    ttlSeconds = math.floor((ttl + 999) / 1000)
end

return {0, nodeId, ttlSeconds}
`)
)

// genActorRegKey 生成Actor的注册key.
func genActorRegKey(uid ActorUID) string {
	return fmt.Sprintf("actor_reg:%d:%d", uid.Category, uid.ID)
}

// genActorUIDPayload 生成跨 Store 复用的 ActorUID 消息载荷。
func genActorUIDPayload(uid ActorUID) string {
	return fmt.Sprintf("%d:%d", uid.Category, uid.ID)
}

// parseActorUIDPayload 解析跨 Store 复用的 ActorUID 消息载荷。
func parseActorUIDPayload(payload string) (ActorUID, error) {
	categoryText, idText, ok := strings.Cut(payload, ":")
	if !ok {
		return ActorUID{}, fmt.Errorf("invalid actor uid payload %q", payload)
	}

	category, err := strconv.ParseInt(categoryText, 10, 64)
	if err != nil {
		return ActorUID{}, pkgerrors.WithMessagef(err, "parse actor uid payload category: %q", payload)
	}
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		return ActorUID{}, pkgerrors.WithMessagef(err, "parse actor uid payload id: %q", payload)
	}
	return ActorUID{
		Category: gactor.ActorCategory(category),
		ID:       gactor.ActorID(id),
	}, nil
}

// RegistryConfig 注册表配置.
type RegistryConfig struct {
	// Redis 客户端.
	RedisCli redis.UniversalClient

	// ChangeStreamMaxLen 是 Location 变更 Stream 的最大近似长度。
	// 应覆盖本地缓存和迁移转发窗口内可能产生的位置变更量。
	// 默认值为 100000。
	ChangeStreamMaxLen int64

	// LocalCacheMaxTTL 是本地缓存允许保留的最长时间。
	// Stream 会主动失效变更位置；TTL 与迁移转发窗口共同覆盖传播延迟。
	// 默认值为 5 秒。
	LocalCacheMaxTTL time.Duration

	// LocalCacheSafetyWindow 用于让本地缓存比 Redis 注册信息更早过期。
	// 默认值为 1 秒。
	LocalCacheSafetyWindow time.Duration
}

func (c *RegistryConfig) init() error {
	if c.RedisCli == nil {
		return errors.New("redis client is nil")
	}
	if c.ChangeStreamMaxLen <= 0 {
		c.ChangeStreamMaxLen = 100000
	}
	if c.LocalCacheMaxTTL <= 0 {
		c.LocalCacheMaxTTL = 5 * time.Second
	}
	if c.LocalCacheSafetyWindow <= 0 {
		c.LocalCacheSafetyWindow = 1 * time.Second
	}
	return nil
}

// Registry Actor 注册表.
type Registry struct {
	cfg        *RegistryConfig    // 配置
	sf         singleflight.Group // 用于并发控制
	locCache   sync.Map           // 本地缓存
	streamCtx  context.Context    // Stream 消费生命周期。
	stopStream context.CancelFunc // 停止 Stream 消费。
	streamWG   sync.WaitGroup     // 等待 Stream 消费退出。
}

// registryLocationCacheEntry 表示一条本地缓存的 Actor Location。
type registryLocationCacheEntry struct {
	location gactor.ActorLocation
	expireAt time.Time
}

// NewRegistry 创建注册表.
func NewRegistry(cfg *RegistryConfig) (*Registry, error) {
	if err := cfg.init(); err != nil {
		return nil, err
	}
	streamCtx, stopStream := context.WithCancel(context.Background())
	reg := &Registry{
		cfg:        cfg,
		streamCtx:  streamCtx,
		stopStream: stopStream,
	}
	reg.startChangeConsumer()
	return reg, nil
}

// Close 停止 Registry 的后台 Stream 消费。
func (d *Registry) Close() error {
	d.stopStream()
	d.streamWG.Wait()
	return nil
}

// MakeLeaseID 生成全局唯一租约 ID.
func (d *Registry) MakeLeaseID() string {
	return xid.New().String()
}

// RegisterActor 注册 Actor.
func (d *Registry) RegisterActor(params gactor.ActorRegisterParams) (gactor.ActorRegisterResult, error) {
	if err := validateRegister(params); err != nil {
		return gactor.ActorRegisterResult{}, err
	}

	regKey := genActorRegKey(params.UID)
	ctx, cancel := newRegistryContext()
	defer cancel()

	result, err := registerActorScript.Run(ctx, d.cfg.RedisCli, []string{regKey},
		params.NodeId,
		params.LeaseId,
		params.TTL,
	).Result()
	if err != nil {
		return gactor.ActorRegisterResult{}, err
	}

	results, err := scriptResults(result)
	if err != nil {
		return gactor.ActorRegisterResult{}, err
	}

	code, err := resultInt64(results, 0, "register")
	if err != nil {
		return gactor.ActorRegisterResult{}, err
	}

	switch code {
	case registryScriptResultOK:
		nodeId, err := resultString(results, 1, "register")
		if err != nil {
			return gactor.ActorRegisterResult{}, err
		}
		ttl, err := resultInt64(results, 2, "register")
		if err != nil {
			return gactor.ActorRegisterResult{}, err
		}
		registerResult := gactor.ActorRegisterResult{
			NodeId:   nodeId,
			ExpireAt: ttlToExpireAt(ttl),
		}
		d.cacheLocation(params.UID, gactor.ActorLocation{
			NodeId:   registerResult.NodeId,
			ExpireAt: registerResult.ExpireAt,
		})
		return registerResult, nil
	case registryScriptResultAlreadyRegistered:
		nodeId, err := resultString(results, 1, "register")
		if err != nil {
			return gactor.ActorRegisterResult{}, err
		}
		ttl, err := resultInt64(results, 2, "register")
		if err != nil {
			return gactor.ActorRegisterResult{}, err
		}
		registerResult := gactor.ActorRegisterResult{
			NodeId:   nodeId,
			ExpireAt: ttlToExpireAt(ttl),
		}
		d.cacheLocation(params.UID, gactor.ActorLocation{
			NodeId:   registerResult.NodeId,
			ExpireAt: registerResult.ExpireAt,
		})
		return registerResult, gactor.ErrActorAlreadyRegistered
	default:
		return gactor.ActorRegisterResult{}, fmt.Errorf("unexpected register script result code %d", code)
	}
}

// TakeOverActor 基于源租约原子接管 Actor。
func (d *Registry) TakeOverActor(params gactor.ActorTakeOverParams) (gactor.ActorRegisterResult, error) {
	if err := validateTakeOver(params); err != nil {
		return gactor.ActorRegisterResult{}, err
	}

	regKey := genActorRegKey(params.UID)
	ctx, cancel := newRegistryContext()
	defer cancel()

	result, err := takeOverActorScript.Run(ctx, d.cfg.RedisCli, []string{regKey, registryChangeStream},
		params.NodeId,
		params.LeaseId,
		params.SourceLeaseId,
		params.TTL,
		d.cfg.ChangeStreamMaxLen,
		params.UID.Category,
		params.UID.ID,
	).Result()
	if err != nil {
		return gactor.ActorRegisterResult{}, err
	}

	results, err := scriptResults(result)
	if err != nil {
		return gactor.ActorRegisterResult{}, err
	}

	code, err := resultInt64(results, 0, "take over")
	if err != nil {
		return gactor.ActorRegisterResult{}, err
	}

	switch code {
	case registryScriptResultOK:
		nodeId, err := resultString(results, 1, "take over")
		if err != nil {
			return gactor.ActorRegisterResult{}, err
		}
		ttl, err := resultInt64(results, 2, "take over")
		if err != nil {
			return gactor.ActorRegisterResult{}, err
		}
		registerResult := gactor.ActorRegisterResult{
			NodeId:   nodeId,
			ExpireAt: ttlToExpireAt(ttl),
		}
		d.cacheLocation(params.UID, gactor.ActorLocation{
			NodeId:   registerResult.NodeId,
			ExpireAt: registerResult.ExpireAt,
		})
		return registerResult, nil
	case registryScriptResultNotExists:
		d.deleteCachedLocation(params.UID)
		return gactor.ActorRegisterResult{}, gactor.ErrActorNotExists
	case registryScriptResultLeaseMismatch:
		return gactor.ActorRegisterResult{}, gactor.ErrLeaseMismatch
	default:
		return gactor.ActorRegisterResult{}, fmt.Errorf("unexpected take over script result code %d", code)
	}
}

// UnregisterActor 注销 Actor.
func (d *Registry) UnregisterActor(params gactor.ActorUnregisterParams) error {
	if err := validateUnregister(params); err != nil {
		return err
	}

	regKey := genActorRegKey(params.UID)
	ctx, cancel := newRegistryContext()
	defer cancel()

	result, err := unregisterActorScript.Run(ctx, d.cfg.RedisCli, []string{regKey, registryChangeStream},
		params.NodeId,
		params.LeaseId,
		d.cfg.ChangeStreamMaxLen,
		params.UID.Category,
		params.UID.ID,
	).Result()
	if err != nil {
		return err
	}

	results, err := scriptResults(result)
	if err != nil {
		return err
	}

	code, err := resultInt64(results, 0, "unregister")
	if err != nil {
		return err
	}

	switch code {
	case registryScriptResultOK:
		d.deleteCachedLocation(params.UID)
		return nil
	case registryScriptResultNotExists:
		d.deleteCachedLocation(params.UID)
		return gactor.ErrActorNotExists
	case registryScriptResultLeaseMismatch:
		return gactor.ErrLeaseMismatch
	default:
		return fmt.Errorf("unexpected unregister script result code %d", code)
	}
}

// KeepActorAlive 保持 Actor 存续.
func (d *Registry) KeepActorAlive(params gactor.ActorKeepAliveParams) error {
	if err := validateKeepAlive(params); err != nil {
		return err
	}

	regKey := genActorRegKey(params.UID)
	ctx, cancel := newRegistryContext()
	defer cancel()

	result, err := keepAliveActorScript.Run(ctx, d.cfg.RedisCli, []string{regKey},
		params.NodeId,
		params.LeaseId,
		params.TTL,
	).Result()
	if err != nil {
		return err
	}

	results, err := scriptResults(result)
	if err != nil {
		return err
	}

	code, err := resultInt64(results, 0, "keepalive")
	if err != nil {
		return err
	}

	switch code {
	case registryScriptResultOK:
		d.cacheLocation(params.UID, gactor.ActorLocation{
			NodeId:   params.NodeId,
			ExpireAt: ttlToExpireAt(params.TTL),
		})
		return nil
	case registryScriptResultNotExists:
		d.deleteCachedLocation(params.UID)
		return gactor.ErrActorNotExists
	case registryScriptResultLeaseMismatch:
		return gactor.ErrLeaseMismatch
	default:
		return fmt.Errorf("unexpected keepalive script result code %d", code)
	}
}

// GetActorLocation 获取 Actor 注册信息.
func (d *Registry) GetActorLocation(uid gactor.ActorUID) (gactor.ActorLocation, error) {
	if err := validateLookup(uid); err != nil {
		return gactor.ActorLocation{}, err
	}

	if location, ok := d.getCachedLocation(uid); ok {
		return location, nil
	}

	regKey := genActorRegKey(uid)
	result, err, _ := d.sf.Do(regKey, func() (any, error) {
		// 双检，避免多个并发请求在等待期间重复回源。
		if location, ok := d.getCachedLocation(uid); ok {
			return location, nil
		}

		location, err := d.getLocationFromRedis(regKey)
		if err != nil {
			return nil, err
		}
		d.cacheLocation(uid, location)
		return location, nil
	})
	if err != nil {
		return gactor.ActorLocation{}, err
	}

	location, ok := result.(gactor.ActorLocation)
	if !ok {
		return gactor.ActorLocation{}, fmt.Errorf("unexpected lookup result type %T", result)
	}
	return location, nil
}

// getLocationFromRedis 从 Redis 注册表中读取 Actor Location。
func (d *Registry) getLocationFromRedis(regKey string) (gactor.ActorLocation, error) {
	ctx, cancel := newRegistryContext()
	defer cancel()

	result, err := getActorRegScript.Run(ctx, d.cfg.RedisCli, []string{regKey}).Result()
	if err != nil {
		return gactor.ActorLocation{}, err
	}

	results, err := scriptResults(result)
	if err != nil {
		return gactor.ActorLocation{}, err
	}

	code, err := resultInt64(results, 0, "lookup")
	if err != nil {
		return gactor.ActorLocation{}, err
	}

	switch code {
	case registryScriptResultOK:
		nodeId, err := resultString(results, 1, "lookup")
		if err != nil {
			return gactor.ActorLocation{}, err
		}
		ttl, err := resultInt64(results, 2, "lookup")
		if err != nil {
			return gactor.ActorLocation{}, err
		}
		return gactor.ActorLocation{
			NodeId:   nodeId,
			ExpireAt: ttlToExpireAt(ttl),
		}, nil
	case registryScriptResultNotExists:
		return gactor.ActorLocation{}, gactor.ErrActorNotExists
	default:
		return gactor.ActorLocation{}, fmt.Errorf("unexpected lookup script result code %d", code)
	}
}

// startChangeConsumer 启动可回放的 Location 变更消费者。
func (d *Registry) startChangeConsumer() {
	d.streamWG.Add(1)
	go func() {
		defer d.streamWG.Done()
		d.consumeLocationChanges()
	}()
}

// consumeLocationChanges 在断线后从最后成功处理的位置回放 Stream。
// 缓存始终可用，迁移源节点的转发窗口覆盖事件传播延迟。
func (d *Registry) consumeLocationChanges() {
	lastID := "$"
	for {
		if d.streamCtx.Err() != nil {
			return
		}

		streams, err := d.cfg.RedisCli.XRead(d.streamCtx, &redis.XReadArgs{
			Streams: []string{registryChangeStream, lastID},
			Count:   256,
			Block:   registryStreamReadBlock,
		}).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			if isMissingStreamError(err) {
				// Stream 首次创建前的缓存可能已存在；从首条事件补读，避免漏掉创建瞬间的迁移。
				lastID = "0-0"
			}
			if !d.waitChangeRetry() {
				return
			}
			continue
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				_ = d.applyLocationChange(message)
				lastID = message.ID
			}
		}
	}
}

// isMissingStreamError 判断 Redis 是否因 Stream 尚未创建而返回错误。
func isMissingStreamError(err error) bool {
	return errors.Is(err, redis.Nil) || (err != nil && strings.Contains(err.Error(), "no such key"))
}

// applyLocationChange 应用一条 Location 变更。缓存只保存位置，不将 Stream 作为第二事实源。
func (d *Registry) applyLocationChange(message redis.XMessage) error {
	category, err := streamInt64(message.Values, "category")
	if err != nil {
		return err
	}
	id, err := streamInt64(message.Values, "id")
	if err != nil {
		return err
	}
	op, err := streamString(message.Values, "op")
	if err != nil {
		return err
	}
	uid := ActorUID{
		Category: gactor.ActorCategory(category),
		ID:       gactor.ActorID(id),
	}
	if op != "upsert" && op != "delete" {
		return fmt.Errorf("unknown registry change operation %q", op)
	}
	d.deleteCachedLocation(uid)
	return nil
}

// waitChangeRetry 等待下一次 Stream 连接尝试。
func (d *Registry) waitChangeRetry() bool {
	select {
	case <-d.streamCtx.Done():
		return false
	case <-time.After(registryStreamReadBlock):
		return true
	}
}

// streamString 从 Redis Stream 字段中读取字符串。
func streamString(values map[string]any, key string) (string, error) {
	value, ok := values[key]
	if !ok {
		return "", fmt.Errorf("registry change field %q is missing", key)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("registry change field %q has unexpected type %T", key, value)
	}
	return text, nil
}

// streamInt64 从 Redis Stream 字段中读取 int64。
func streamInt64(values map[string]any, key string) (int64, error) {
	text, err := streamString(values, key)
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, pkgerrors.WithMessagef(err, "parse registry change field %q", key)
	}
	return value, nil
}

// getCachedLocation 尝试读取本地缓存中的 Actor Location。
func (d *Registry) getCachedLocation(uid ActorUID) (gactor.ActorLocation, bool) {
	value, ok := d.locCache.Load(uid)
	if !ok {
		return gactor.ActorLocation{}, false
	}

	entry, ok := value.(registryLocationCacheEntry)
	if !ok {
		d.locCache.Delete(uid)
		return gactor.ActorLocation{}, false
	}
	if !entry.expireAt.After(time.Now()) {
		d.locCache.Delete(uid)
		return gactor.ActorLocation{}, false
	}
	return entry.location, true
}

// cacheLocation 将 Actor Location 写入本地短缓存。
func (d *Registry) cacheLocation(uid ActorUID, location gactor.ActorLocation) {
	now := time.Now()
	localTTL := d.calcLocalCacheTTL(location, now)
	if localTTL <= 0 {
		d.deleteCachedLocation(uid)
		return
	}

	d.locCache.Store(uid, registryLocationCacheEntry{
		location: location,
		expireAt: now.Add(localTTL),
	})
}

// deleteCachedLocation 删除本地缓存中的 Actor Location。
func (d *Registry) deleteCachedLocation(uid ActorUID) {
	d.locCache.Delete(uid)
}

// calcLocalCacheTTL 计算本地缓存 TTL。
// 本地缓存必须比 Redis 记录更早过期，这样即使漏掉失效通知，也会很快回源纠正。
func (d *Registry) calcLocalCacheTTL(location gactor.ActorLocation, now time.Time) time.Duration {
	localTTL := d.cfg.LocalCacheMaxTTL
	if location.ExpireAt <= 0 {
		return localTTL
	}

	remainingTTL := time.Unix(location.ExpireAt, 0).Sub(now)
	if remainingTTL <= d.cfg.LocalCacheSafetyWindow {
		return 0
	}
	remainingTTL -= d.cfg.LocalCacheSafetyWindow
	if remainingTTL < localTTL {
		return remainingTTL
	}
	return localTTL
}

func newRegistryContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), registryTimeout)
}

func ttlToExpireAt(ttl int64) int64 {
	if ttl <= 0 {
		return 0
	}
	return time.Now().Unix() + ttl
}

func validateUID(uid ActorUID) error {
	if uid.IsZero() {
		return ErrActorUIDRequired
	}
	return nil
}

func validateRegister(params gactor.ActorRegisterParams) error {
	if err := validateUID(params.UID); err != nil {
		return pkgerrors.WithMessage(err, "register actor")
	}
	if params.NodeId == "" {
		return pkgerrors.WithMessage(ErrActorNodeIDRequired, "register actor")
	}
	if params.LeaseId == "" {
		return pkgerrors.WithMessage(ErrActorLeaseIDRequired, "register actor")
	}
	if params.TTL < 0 {
		return pkgerrors.WithMessagef(ErrActorTTLInvalid, "register actor: invalid ttl %d", params.TTL)
	}
	return nil
}

func validateTakeOver(params gactor.ActorTakeOverParams) error {
	if err := validateUID(params.UID); err != nil {
		return pkgerrors.WithMessage(err, "take over actor")
	}
	if params.NodeId == "" {
		return pkgerrors.WithMessage(ErrActorNodeIDRequired, "take over actor")
	}
	if params.LeaseId == "" {
		return pkgerrors.WithMessage(ErrActorLeaseIDRequired, "take over actor")
	}
	if params.SourceLeaseId == "" {
		return pkgerrors.WithMessage(ErrActorSourceLeaseIDRequired, "take over actor")
	}
	if params.TTL < 0 {
		return pkgerrors.WithMessagef(ErrActorTTLInvalid, "take over actor: invalid ttl %d", params.TTL)
	}
	return nil
}

func validateUnregister(params gactor.ActorUnregisterParams) error {
	if err := validateUID(params.UID); err != nil {
		return pkgerrors.WithMessage(err, "unregister actor")
	}
	if params.NodeId == "" {
		return pkgerrors.WithMessage(ErrActorNodeIDRequired, "unregister actor")
	}
	if params.LeaseId == "" {
		return pkgerrors.WithMessage(ErrActorLeaseIDRequired, "unregister actor")
	}
	return nil
}

func validateKeepAlive(params gactor.ActorKeepAliveParams) error {
	if err := validateUID(params.UID); err != nil {
		return pkgerrors.WithMessage(err, "keep actor alive")
	}
	if params.NodeId == "" {
		return pkgerrors.WithMessage(ErrActorNodeIDRequired, "keep actor alive")
	}
	if params.LeaseId == "" {
		return pkgerrors.WithMessage(ErrActorLeaseIDRequired, "keep actor alive")
	}
	if params.TTL < 0 {
		return pkgerrors.WithMessagef(ErrActorTTLInvalid, "keep actor alive: invalid ttl %d", params.TTL)
	}
	return nil
}

func validateLookup(uid ActorUID) error {
	if err := validateUID(uid); err != nil {
		return pkgerrors.WithMessage(err, "lookup actor registry")
	}
	return nil
}

func scriptResults(result any) ([]any, error) {
	results, ok := result.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected script result type %T", result)
	}
	if len(results) == 0 {
		return nil, errors.New("empty script result")
	}
	return results, nil
}

func resultAt(results []any, index int, op string) (any, error) {
	if index < 0 || index >= len(results) {
		return nil, fmt.Errorf("%s script result too short: got %d values, need index %d", op, len(results), index)
	}
	return results[index], nil
}

func parseInt64(v any) (int64, error) {
	switch v := v.(type) {
	case int64:
		return v, nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	case []byte:
		return strconv.ParseInt(string(v), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected script integer type %T", v)
	}
}

func parseString(v any) (string, error) {
	switch v := v.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("unexpected script string type %T", v)
	}
}

func resultInt64(results []any, index int, op string) (int64, error) {
	v, err := resultAt(results, index, op)
	if err != nil {
		return 0, err
	}
	return parseInt64(v)
}

func resultString(results []any, index int, op string) (string, error) {
	v, err := resultAt(results, index, op)
	if err != nil {
		return "", err
	}
	return parseString(v)
}
