package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/adarsh-jaiss/segwise/internal/cache"
	store "github.com/adarsh-jaiss/segwise/internal/db"
	"github.com/adarsh-jaiss/segwise/internal/worker"

	"github.com/adarsh-jaiss/segwise/internal/service"
	"github.com/adarsh-jaiss/segwise/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	DB    *sql.DB
	Redis *redis.Client
}

func NewHandler(db *sql.DB, redis *redis.Client) *Handler {
	return &Handler{
		DB:    db,
		Redis: redis,
	}
}

func (h *Handler) hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}

func (h *Handler) createSubscriptions(db *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		var subscriptions types.Subscription
		if err := c.Bind(&subscriptions); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid request payload")
		}

		subscriptions.ID = uuid.New()
		subscriptions.Active = true

		response, err := store.CreateSubscriptionsInDB(db, subscriptions)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("Failed to create subscription: %v", err))
		}

		err = sendTaskToQueue(response, subscriptions.Payload)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("error sending task to queue : %v", err))
		}

		// Implement the logic to create subscriptions
		return c.JSON(http.StatusAccepted, map[string]interface{}{
			"message": "Subscription created",
			"id":      response,
		})
	}
}

func (h *Handler) getSubscriptions(db *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		id := c.Param("id")
		if id == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "ID is required")
		}

		subscription, err := store.GetSubscriptionByIDFromStore(db, id)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get subscription")
		}

		if subscription == nil {
			return echo.NewHTTPError(http.StatusNotFound, "Subscription not found")
		}

		return c.JSON(http.StatusOK, subscription)
	}
}

func (h *Handler) deleteSubscriptions(db *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		id := c.Param("id")
		if id == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "ID is required")
		}

		err := store.DeleteSubscriptionByIDFromStore(db, id)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete subscription")
		}

		// Delete from Redis cache
		err = cache.DeleteSubscriptionCache(h.Redis, id)

		return c.JSON(http.StatusOK, "Subscription deleted")
	}
}

func (h *Handler) updateSubscriptions(db *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		id := c.Param("id")
		if id == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "ID is required")
		}

		var subscription types.Subscription
		if err := c.Bind(&subscription); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid request payload")
		}

		parsedID, err := uuid.Parse(id)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid UUID format")
		}
		subscription.ID = parsedID

		if err := store.UpdateSubscriptionInDB(db, subscription); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update subscription")
		}

		return c.JSON(http.StatusAccepted, "Subscription updated")
	}
}

// IngestTask handles incoming webhook requests and queues them for processing
func (h *Handler) IngestTask(redis *redis.Client) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Get subscription ID from path parameter
		subscriptionID := c.Param("id")
		if subscriptionID == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Subscription ID is required")
		}

		// Get subscription details from cache or database
		subscription, err := cache.GetSubscriptionCache(redis, subscriptionID)
		if err != nil {
			subscription, err = service.FetchAndCacheSubscription(h.DB, h.Redis, subscriptionID)
			if err != nil {
				return echo.NewHTTPError(http.StatusNotFound, fmt.Errorf("Subscription not found: %v", err))
			}
		}

		// Read request body
		body, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("Failed to read request body: %v", err))
		}

		// Parse the payload as JSON
		var payload json.RawMessage
		if err := json.Unmarshal(body, &payload); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("Invalid JSON payload: %v", err))
		}

		// CRITICAL: Make sure GlobalScheduler is not nil before using it
		if worker.GlobalScheduler == nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Webhook scheduler not initialized")
		}

		// Process the webhook request using our scheduler
		taskID := worker.GlobalScheduler.ProcessRequest(
			subscriptionID,
			subscription.TargetURL,
			subscription.Secret,
			payload,
		)

		// Return immediate response with task ID
		return c.JSON(http.StatusAccepted, map[string]string{
			"message": "webhook delivery queued",
			"task_id": taskID,
		})
	}
}

// GetTaskStatus retrieves the status of a specific task
func (h *Handler) GetTaskStatus(redis *redis.Client) echo.HandlerFunc {
	return func(c echo.Context) error {
		taskID := c.Param("id")
		if taskID == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Task ID is required")
		}

		if worker.GlobalScheduler == nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Webhook scheduler not initialized")
		}

		task, err := worker.GlobalScheduler.GetTaskStatus(taskID)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, fmt.Errorf("Task not found: %v", err))
		}

		return c.JSON(http.StatusOK, task)
	}
}

// GetSubscriptionTasks retrieves all tasks for a subscription
func (h *Handler) GetSubscriptionTasks(redis *redis.Client) echo.HandlerFunc {
	return func(c echo.Context) error {
		subscriptionID := c.Param("id")
		if subscriptionID == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Subscription ID is required")
		}

		if worker.GlobalScheduler == nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Webhook scheduler not initialized")
		}

		tasks, err := worker.GlobalScheduler.GetSubscriptionTasks(subscriptionID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("Failed to retrieve tasks: %v", err))
		}

		return c.JSON(http.StatusOK, tasks)
	}
}

// GetSubscriptionLogs retrieves all logs for a subscription
func (h *Handler) GetSubscriptionLogs(db *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		subscriptionID := c.Param("id")
		if subscriptionID == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Subscription ID is required")
		}

		// Convert string ID to UUID
		subscriptionUUID, err := uuid.Parse(subscriptionID)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid subscription ID format")
		}

		// Create a log service
		logService := service.NewLogService(db)

		// Get logs for the subscription
		logs, err := logService.GetLogsBySubscriptionID(c.Request().Context(), subscriptionUUID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("Failed to retrieve logs: %v", err))
		}

		return c.JSON(http.StatusOK, logs)
	}
}



// GetRecentDeliveryAttempts retrieves the most recent delivery attempts for a subscription
func (h *Handler) GetRecentDeliveryAttempts(db *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		subscriptionID := c.Param("id")
		if subscriptionID == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Subscription ID is required")
		}

		// Get limit parameter, default to 20
		limitStr := c.QueryParam("limit")
		limit := 20
		if limitStr != "" {
			fmt.Sscanf(limitStr, "%d", &limit)
			if limit <= 0 {
				limit = 20
			}
		}

		// Convert string ID to UUID
		subscriptionUUID, err := uuid.Parse(subscriptionID)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid subscription ID format")
		}

		// Execute a custom query to get only the most recent attempts
		query := `
			SELECT id, task_id, subscription_id, target_url, timestamp, attempt_number, 
				status, status_code, error_details, created_at, updated_at
			FROM logs 
			WHERE subscription_id = $1
			ORDER BY timestamp DESC
			LIMIT $2
		`

		rows, err := db.QueryContext(c.Request().Context(), query, subscriptionUUID, limit)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("Failed to query logs: %v", err))
		}
		defer rows.Close()

		var logs []types.Logs
		for rows.Next() {
			var log types.Logs
			err := rows.Scan(
				&log.ID,
				&log.TaskID,
				&log.SubscriptionID,
				&log.TargetURL,
				&log.Timestamp,
				&log.AttemptNumber,
				&log.Status,
				&log.StatusCode,
				&log.ErrorDetails,
				&log.CreatedAt,
				&log.UpdatedAt,
			)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("Error scanning log row: %v", err))
			}
			logs = append(logs, log)
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"subscription_id": subscriptionID,
			"count":           len(logs),
			"limit":           limit,
			"attempts":        logs,
		})
	}
}

func sendTaskToQueue(taskID string, payload interface{}) error {
	// Create HTTP client
	client := &http.Client{}

	// Create request body with just the payload
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}

	// Create request with taskID in path parameter
	req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:8080/api/ingest/%s", taskID), bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	fmt.Println("Response Status:", resp.Status)
	// Check response status
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
