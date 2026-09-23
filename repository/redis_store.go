package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
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

func (r *RedisStore) SaveBucket(ctx context.Context,clientId string, bucket model.Bucket)(error){

	key := "rate-limit" +clientId

	err := r.client.HSet(ctx,key,map[string]interface{}{
		"capacit" : bucket.Capacity,
		"refillRate" : bucket.RefillRate,
		"currentTokens": bucket.CurrentTokens,
		"lastRefill": bucket.LastRefill.UnixNano(),
	}).Err()
	return err
}

func(r *RedisStore) GetBucket(ctx context.Context,clientId string)(model.Bucket,error){
	key := "rate-limit"+clientId

	data,err := r.client.HGetAll(ctx,key).Result()
	if err != nil {
		return model.Bucket{},err
	}

	if len(data)==0 {
		return model.Bucket{},redis.Nil
	}
	lastRefill,err :=strconv.ParseInt(data["lastRefill"],10,64)
	if err != nil {
		return model.Bucket{},err
	}

	capacity,err := strconv.ParseFloat(data["capacity"],64)
	if err != nil{
		return model.Bucket{},err
	}


	refillRate, err := strconv.ParseFloat(data["refillRate"], 64)
	if err != nil {
		return model.Bucket{}, err
	}

	currentTokens, err := strconv.ParseFloat(data["currentTokens"], 64)
	if err != nil {
		return model.Bucket{}, err
	}

	return model.Bucket{
		Capacity:      capacity,
		RefillRate:    refillRate,
		CurrentTokens: currentTokens,
		LastRefill:    time.Unix(0, lastRefill),
	}, nil
}
