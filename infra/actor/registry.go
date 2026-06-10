package actor

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/godyy/gactor"
	pkgerrors "github.com/pkg/errors"
	redis "github.com/redis/go-redis/v9"
	"github.com/rs/xid"
)

const registryTimeout = 5 * time.Second

const (
	registryScriptResultOK = iota
	registryScriptResultNotExists
	registryScriptResultAlreadyRegistered
	registryScriptResultLeaseMismatch
)

var (
	ErrActorUIDRequired     = errors.New("actor uid is required")
	ErrActorNodeIDRequired  = errors.New("node id is required")
	ErrActorLeaseIDRequired = errors.New("lease id is required")
	ErrActorTTLInvalid      = errors.New("ttl must be greater than or equal to 0")

	registerActorScript = redis.NewScript(`
local raw = redis.call("GET", KEYS[1])

if raw then
    local current = cjson.decode(raw)
    local currentNodeId = current["NodeId"] or ""
    local currentTTL = redis.call("PTTL", KEYS[1])

    if currentTTL > 0 then
        currentTTL = math.floor((currentTTL + 999) / 1000)
    end

    if currentNodeId ~= "" and currentNodeId ~= ARGV[3] then
        return {2, currentNodeId, currentTTL}
    end
end

local reg = {
    UID = {
        Category = tonumber(ARGV[1]),
        ID = tonumber(ARGV[2]),
    },
    NodeId = ARGV[3],
    LeaseId = ARGV[4],
}

local ttl = tonumber(ARGV[5]) or 0
if ttl > 0 then
    redis.call("SET", KEYS[1], cjson.encode(reg), "EX", ttl)
    return {0, reg["NodeId"], ttl}
end

redis.call("SET", KEYS[1], cjson.encode(reg))
return {0, reg["NodeId"], 0}
`)

	unregisterActorScript = redis.NewScript(`
local raw = redis.call("GET", KEYS[1])
if not raw then
    return {1}
end

local reg = cjson.decode(raw)

if (reg["NodeId"] or "") ~= ARGV[1] or (reg["LeaseId"] or "") ~= ARGV[2] then
    return {3}
end

redis.call("DEL", KEYS[1])
return {0}
`)

	keepAliveActorScript = redis.NewScript(`
local raw = redis.call("GET", KEYS[1])
if not raw then
    return {1}
end

local reg = cjson.decode(raw)

if (reg["NodeId"] or "") ~= ARGV[1] or (reg["LeaseId"] or "") ~= ARGV[2] then
    return {3}
end

local ttl = tonumber(ARGV[3]) or 0
if ttl > 0 then
    redis.call("SET", KEYS[1], cjson.encode(reg), "EX", ttl)
else
    redis.call("SET", KEYS[1], cjson.encode(reg))
end

return {0}
`)

	getActorRegScript = redis.NewScript(`
local raw = redis.call("GET", KEYS[1])
if not raw then
    return {1}
end

local reg = cjson.decode(raw)
local ttlSeconds = 0
local ttl = redis.call("PTTL", KEYS[1])
if ttl > 0 then
    ttlSeconds = math.floor((ttl + 999) / 1000)
end

return {0, reg["NodeId"] or "", ttlSeconds}
`)
)

// genActorRegKey 生成Actor的注册key.
func genActorRegKey(uid gactor.ActorUID) string {
	return fmt.Sprintf("actor_reg:%d:%d", uid.Category, uid.ID)
}

// Registry Actor 注册表.
type Registry struct {
	redisCli redis.UniversalClient
}

// NewRegistry 创建注册表.
func NewRegistry(redisCli redis.UniversalClient) (*Registry, error) {
	if redisCli == nil {
		return nil, errors.New("redis client is nil")
	}
	return &Registry{redisCli: redisCli}, nil
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

	ctx, cancel := newRegistryContext()
	defer cancel()

	result, err := registerActorScript.Run(ctx, d.redisCli, []string{genActorRegKey(params.UID)},
		params.UID.Category,
		params.UID.ID,
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
		return gactor.ActorRegisterResult{
			NodeId:   nodeId,
			ExpireAt: ttlToExpireAt(ttl),
		}, nil
	case registryScriptResultAlreadyRegistered:
		nodeId, err := resultString(results, 1, "register")
		if err != nil {
			return gactor.ActorRegisterResult{}, err
		}
		ttl, err := resultInt64(results, 2, "register")
		if err != nil {
			return gactor.ActorRegisterResult{}, err
		}
		return gactor.ActorRegisterResult{
			NodeId:   nodeId,
			ExpireAt: ttlToExpireAt(ttl),
		}, gactor.ErrActorAlreadyRegistered
	default:
		return gactor.ActorRegisterResult{}, fmt.Errorf("unexpected register script result code %d", code)
	}
}

// UnregisterActor 注销 Actor.
func (d *Registry) UnregisterActor(params gactor.ActorUnregisterParams) error {
	if err := validateUnregister(params); err != nil {
		return err
	}

	ctx, cancel := newRegistryContext()
	defer cancel()

	result, err := unregisterActorScript.Run(ctx, d.redisCli, []string{genActorRegKey(params.UID)},
		params.NodeId,
		params.LeaseId,
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
		return nil
	case registryScriptResultNotExists:
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

	ctx, cancel := newRegistryContext()
	defer cancel()

	result, err := keepAliveActorScript.Run(ctx, d.redisCli, []string{genActorRegKey(params.UID)},
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
		return nil
	case registryScriptResultNotExists:
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

	ctx, cancel := newRegistryContext()
	defer cancel()

	result, err := getActorRegScript.Run(ctx, d.redisCli, []string{genActorRegKey(uid)}).Result()
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

func newRegistryContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), registryTimeout)
}

func ttlToExpireAt(ttl int64) int64 {
	if ttl <= 0 {
		return 0
	}
	return time.Now().Unix() + ttl
}

func validateUID(uid gactor.ActorUID) error {
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

func validateLookup(uid gactor.ActorUID) error {
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
