package redis

import (
	"context"
	"errors"
	"time"

	pkgerrors "github.com/pkg/errors"
	goredis "github.com/redis/go-redis/v9"
)

// Config 映射 redis.Client 配置.
type Config struct {
	// Addrs 实例地址.
	Addrs []string

	// UserName 用户名.
	Username string

	// Password 密码.
	Password string

	// DB 数据库编号.
	DB int

	// DialTimeout 连接超时.
	DialTimeout time.Duration

	// ReadWriteTimeout 连接套接字读写超时.
	ReadWriteTimeout time.Duration

	// PoolSize 连接池大小.
	PoolSize int

	// PoolTimeout 等待连接池可用连接超时.
	PoolTimeout time.Duration

	// MinIdleConns 最小空闲连接数.
	MinIdleConns int

	// MaxIdleConns 最大空闲连接数.
	MaxIdleConns int

	// MaxActiveConns 连接池可以分配的最大连接数.
	MaxActiveConns int

	// ConnMaxIdleTime 空闲连接最大空闲时间.
	ConnMaxIdleTime time.Duration
}

// Client 透传 redis 驱动客户端类型，便于调用方只依赖当前包。
type Client = goredis.UniversalClient

func (cfg *Config) universal() *goredis.UniversalOptions {
	return &goredis.UniversalOptions{
		Addrs:           cfg.Addrs,
		Username:        cfg.Username,
		Password:        cfg.Password,
		DB:              cfg.DB,
		DialTimeout:     cfg.DialTimeout,
		ReadTimeout:     cfg.ReadWriteTimeout,
		WriteTimeout:    cfg.ReadWriteTimeout,
		PoolSize:        cfg.PoolSize,
		PoolTimeout:     cfg.PoolTimeout,
		MinIdleConns:    cfg.MinIdleConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		MaxActiveConns:  cfg.MaxActiveConns,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
	}
}

// NewClient 创建客户端连接.
func NewClient(cfg *Config) (Client, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}

	cli := goredis.NewUniversalClient(cfg.universal())
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout+cfg.ReadWriteTimeout)
	defer cancel()
	if err := cli.Ping(ctx).Err(); err != nil {
		_ = cli.Close()
		return nil, pkgerrors.WithMessage(err, "ping")
	}

	return cli, nil
}
