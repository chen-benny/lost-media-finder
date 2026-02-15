package storage

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

func NewRedis(addr string) (*Redis, error) {
	client := redis.NewClient(&redis.Options{Addr: addr})
	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return nil, err
	}
	return &Redis{client: client}, nil
}

func (r *Redis) TryAdd(ctx context.Context, prefix, url string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, prefix+url, 1, ttl).Result()
}

func (r *Redis) Close() {
	r.client.Close()
}

func (r *Redis) FlushDB(ctx context.Context) error {
	return r.client.FlushDB(ctx).Err()
}

const overflowKey = "crawler:overflow"

func (r *Redis) PushOverflow(ctx context.Context, url string) error {
	return r.client.LPush(ctx, overflowKey, url).Err()
}

func (r *Redis) PopOverflow(ctx context.Context) (string, error) {
	return r.client.RPop(ctx, overflowKey).Result()
}
