package store

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	rdb *redis.Client
}

func NewRedisClient(ctx context.Context, addr, password string, db int) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}

	return &RedisClient{rdb: rdb}, nil
}

func (c *RedisClient) Close() error {
	return c.rdb.Close()
}

func (c *RedisClient) Client() *redis.Client {
	return c.rdb
}
