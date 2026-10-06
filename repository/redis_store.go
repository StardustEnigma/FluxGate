package repository

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore() *RedisStore {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// Fix #1 – Explicitly size the Redis connection pool for high concurrency.
	// With the library default (10 * GOMAXPROCS connections), 200 concurrent
	// workers exhaust the pool and goroutines stall in pool.Get() for up to
	// PoolTimeout (previously 4 s) — the dominant source of p99 tail latency.
	// The Prometheus histogram never captured this wait because it starts only
	// after a connection is already acquired inside Eval().
	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		PoolSize:     250,                    // headroom above 200-worker benchmark
		MinIdleConns: 20,                     // pre-warm to absorb the first burst
		DialTimeout:  500 * time.Millisecond, // fail fast on network issues
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		PoolTimeout:  2 * time.Second, // surface pool exhaustion quickly, not after 4 s
	})

	return &RedisStore{
		client: rdb,
	}
}

func (r *RedisStore) Eval(
	ctx context.Context,
	script string,
	keys []string,
	args ...any,
) (any, error) {
	return r.client.Eval(ctx, script, keys, args...).Result()
}

func (r *RedisStore) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisStore) HSet(ctx context.Context, key string, values ...any) error {
	result := r.client.HSet(ctx, key, values...)
	if err := result.Err(); err != nil {
		return err
	}
	return nil
}

func (r *RedisStore) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	data, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, redis.Nil
	}
	return data, nil
}

func (r *RedisStore) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	return r.client.ZAdd(ctx, key, members...).Err()
}

func (r *RedisStore) ZRemRangeByScore(ctx context.Context, key string, min, max string) error {
	return r.client.ZRemRangeByScore(ctx, key, min, max).Err()
}

func (r *RedisStore) ZCard(ctx context.Context, key string) (int64, error) {
	return r.client.ZCard(ctx, key).Result()
}

func (r *RedisStore) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.ZRange(ctx, key, start, stop).Result()
}

func (r *RedisStore) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	return r.client.ZRangeWithScores(ctx, key, start, stop).Result()
}

func (r *RedisStore) PoolStats() *redis.PoolStats {
	return r.client.PoolStats()
}
