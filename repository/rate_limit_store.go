package repository

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RateLimitStore interface {
	Eval(
		ctx context.Context,
		script string,
		keys []string,
		args ...any,
	) (any, error)

	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HSet(ctx context.Context, key string, value ...any) error

	ZAdd(ctx context.Context, key string, member ...redis.Z) error
	ZRemRangeByScore(ctx context.Context, key string, min, max string) error
	ZCard(ctx context.Context, key string) (int64, error)
	ZRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error)
}
