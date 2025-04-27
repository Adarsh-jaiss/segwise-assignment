package cache

import (
	"context"
	"encoding/json"

	"github.com/adarsh-jaiss/segwise/types"
	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

func SetSubscriptionCache(r *redis.Client, id string, sub types.Subscription) error {
	jsonData, err := json.Marshal(sub)
	if err != nil {
		return err
	}
	err = r.Set(ctx, id, jsonData, 0).Err()
	if err != nil {
		return err
	}
	return nil
}

func GetSubscriptionCache(r *redis.Client, id string) (types.Subscription, error) {
	var sub types.Subscription
	val, err := r.Get(ctx, id).Result()
	if err != nil {
		return sub, err
	}

	err = json.Unmarshal([]byte(val), &sub)
	if err != nil {
		return sub, err
	}
	return sub, nil
}

func DeleteSubscriptionCache(r *redis.Client, id string) error {
	err := r.Del(ctx, id).Err()
	if err != nil {
		return err
	}
	return nil
}

