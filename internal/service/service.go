package service

import (
	"database/sql"

	"github.com/adarsh-jaiss/segwise/internal/cache"
	store "github.com/adarsh-jaiss/segwise/internal/db"
	"github.com/adarsh-jaiss/segwise/types"
	"github.com/redis/go-redis/v9"
	"fmt"
)

func FetchAndCacheSubscription(db *sql.DB, c *redis.Client, id string) 	(types.Subscription, error) {
	// Fetch subscription from DB
	subscription, err := store.GetSubscriptionByIDFromStore(db, id)
	if err != nil {
		return types.Subscription{}, fmt.Errorf("failed to get subscription from DB: %v", err)
	}

	// Cache the subscription
	err = cache.SetSubscriptionCache(c, id, *subscription)
	if err != nil {
		return types.Subscription{}, fmt.Errorf("failed to cache subscription: %v", err)
	}

	return *subscription, nil
}