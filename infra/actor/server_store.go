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
)

var (
	ErrActorServerIDInvalid = errors.New("server id must be greater than 0")
)

const serverStoreTimeout = 5 * time.Second

// genActorServerKey 生成 Actor 所属服务器映射 key.
func genActorServerKey(uid gactor.ActorUID) string {
	return fmt.Sprintf("actor_server:%d:%d", uid.Category, uid.ID)
}

// ServerStore Actor 所属服务器存储.
type ServerStore struct {
	redisCli redis.UniversalClient
}

// NewServerStore 创建 Actor 所属服务器存储.
func NewServerStore(redisCli redis.UniversalClient) (*ServerStore, error) {
	if redisCli == nil {
		return nil, errors.New("redis client is nil")
	}
	return &ServerStore{redisCli: redisCli}, nil
}

// SetActorServer 设置 Actor 所属服务器.
func (s *ServerStore) SetActorServer(uid gactor.ActorUID, serverID int64) error {
	if err := validateUID(uid); err != nil {
		return pkgerrors.WithMessage(err, "set actor server")
	}
	if serverID <= 0 {
		return pkgerrors.WithMessagef(ErrActorServerIDInvalid, "set actor server: invalid server id %d", serverID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), serverStoreTimeout)
	defer cancel()

	if err := s.redisCli.Set(ctx, genActorServerKey(uid), strconv.FormatInt(serverID, 10), 0).Err(); err != nil {
		return pkgerrors.WithMessage(err, "set actor server")
	}
	return nil
}

// GetActorServer 获取 Actor 所属服务器.
func (s *ServerStore) GetActorServer(uid gactor.ActorUID) (int64, bool, error) {
	if err := validateUID(uid); err != nil {
		return 0, false, pkgerrors.WithMessage(err, "get actor server")
	}

	ctx, cancel := context.WithTimeout(context.Background(), serverStoreTimeout)
	defer cancel()

	raw, err := s.redisCli.Get(ctx, genActorServerKey(uid)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, pkgerrors.WithMessage(err, "get actor server")
	}

	serverID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false, pkgerrors.WithMessagef(err, "parse actor server id %q", raw)
	}
	if serverID <= 0 {
		return 0, false, pkgerrors.WithMessagef(ErrActorServerIDInvalid, "parse actor server id %q", raw)
	}

	return serverID, true, nil
}
