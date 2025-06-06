package cache

import (
	"github.com/redis/go-redis/v9"
	"context"
)

func NewRedisClient(addr, password, username string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		Username: username,
		DB:       0,
	})

	// Test the connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}


