package repository

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct{
	client *redis.Client
}

func NewRedisStore() *RedisStore{
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	return &RedisStore{
		client: client,
	}
}

func (r *RedisStore) Ping(ctx context.Context) error{
	return r.client.Ping(ctx).Err()
}

func (r *RedisStore) HSet(ctx context.Context, key string, values ...any) error {

	result := r.client.HSet(ctx, key,values...)

	if err := result.Err(); err != nil {
		return err
	}


	return nil
}

func(r *RedisStore) HGetAll(ctx context.Context,key string)(map[string]string,error){
	
	data,err := r.client.HGetAll(ctx,key).Result()
	if err != nil {
		return nil,err
	}

	if len(data)==0 {
		return nil,redis.Nil
	}
	

	return data, nil
}

func(r * RedisStore)ZAdd(ctx context.Context,key string,members ...redis.Z)error{

	return r.client.ZAdd(ctx,key,members...).Err()
}

func(r *RedisStore)ZRemRangeByScore(ctx context.Context,key string,min,max string)error{
	return r.client.ZRemRangeByScore(ctx,key,min,max).Err()
}
func(r *RedisStore)ZCard(ctx context.Context,key string)(int64,error){
	return r.client.ZCard(ctx,key).Result()
}

func(r *RedisStore)ZRANGE(ctx context.Context,key string,start,stop int64)([]string,error){
	return r.client.ZRange(ctx,key,start,stop).Result()
}
func(r *RedisStore)ZRangeWithScores(ctx context.Context,key string,start,stop int64)([]redis.Z,error){
	return r.client.ZRangeWithScores(ctx,key,start,stop).Result()
}