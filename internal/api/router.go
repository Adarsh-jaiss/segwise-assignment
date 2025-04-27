package api

import (
	"database/sql"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func RegisterRoutes(e *echo.Echo, db *sql.DB, redis *redis.Client) {
	h := NewHandler(db, redis)

	e.GET("/", h.hello)
	e.POST("/api/subscriptions", h.createSubscriptions(db))
	e.GET("/api/subscriptions/:id", h.getSubscriptions(db))
	e.DELETE("/api/subscriptions/:id", h.deleteSubscriptions(db))
	e.PATCH("/api/subscriptions/:id", h.updateSubscriptions(db))

	// Webhook delivery routes
	e.POST("/api/ingest/:id", h.IngestTask(redis))

	// Add these routes for task status management
	e.GET("/api/tasks/:id", h.GetTaskStatus(redis))
	e.GET("/api/subscriptions/:id/tasks", h.GetSubscriptionTasks(redis))

	// Log-related routes
	e.GET("/api/subscriptions/:id/logs", h.GetSubscriptionLogs(db))

	// Analytics routes
	e.GET("/api/analytics/subscriptions/:id/recent-attempts", h.GetRecentDeliveryAttempts(db))
}
