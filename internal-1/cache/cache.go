Sure, here's the contents for the file: /internal/cache/cache.go

package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache provides methods for interacting with the Redis cache.
type Cache struct {
	client *redis.Client
	ctx    context.Context
}

// NewCache creates a new Cache instance with the provided Redis client and context.
func NewCache(client *redis.Client, ctx context.Context) *Cache {
	return &Cache{
		client: client,
		ctx:    ctx,
	}
}

// Set sets a value in the cache with an expiration time.
func (c *Cache) Set(key string, value interface{}, expiration time.Duration) error {
	return c.client.Set(c.ctx, key, value, expiration).Err()
}

// Get retrieves a value from the cache.
func (c *Cache) Get(key string) (string, error) {
	return c.client.Get(c.ctx, key).Result()
}

// Delete removes a value from the cache.
func (c *Cache) Delete(key string) error {
	return c.client.Del(c.ctx, key).Err()
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(key string) (bool, error) {
	count, err := c.client.Exists(c.ctx, key).Result()
	return count > 0, err
}