package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/adarsh-jaiss/segwise/internal/cache"
	"github.com/adarsh-jaiss/segwise/internal/db"
	"github.com/adarsh-jaiss/segwise/internal/worker"
	"github.com/adarsh-jaiss/segwise/types"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

// TestWebhookServer is a simple server that receives webhook deliveries
type TestWebhookServer struct {
	receivedWebhooks      map[string]int
	webhookResponses      map[string]int // Status codes to return for specific subscription IDs
	mutex                 sync.Mutex
	failFirstNDeliveries  int // How many deliveries to fail before succeeding
	currentFailedAttempts int
}

func NewTestWebhookServer() *TestWebhookServer {
	return &TestWebhookServer{
		receivedWebhooks:     make(map[string]int),
		webhookResponses:     make(map[string]int),
		failFirstNDeliveries: 3, // Default to fail first 3 attempts
	}
}

func (s *TestWebhookServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract subscription ID from request
	subscriptionID := r.Header.Get("X-Webhook-ID")
	attempt := r.Header.Get("X-Webhook-Attempt")

	log.Printf("Received webhook delivery for ID: %s, attempt: %s", subscriptionID, attempt)

	// Record this delivery
	s.mutex.Lock()
	s.receivedWebhooks[subscriptionID]++
	attemptCount := s.receivedWebhooks[subscriptionID]
	s.mutex.Unlock()

	// Check if we should return an error based on attempt count
	if attemptCount <= s.failFirstNDeliveries {
		log.Printf("Simulating failure for webhook ID %s (attempt %d of %d to fail)",
			subscriptionID, attemptCount, s.failFirstNDeliveries)
		w.WriteHeader(http.StatusNotFound) // 404
		return
	}

	// Otherwise, return success
	log.Printf("Simulating success for webhook ID %s on attempt %d", subscriptionID, attemptCount)
	w.WriteHeader(http.StatusOK)
}

func (s *TestWebhookServer) SetResponse(subscriptionID string, statusCode int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.webhookResponses[subscriptionID] = statusCode
}

func (s *TestWebhookServer) SetFailCount(count int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.failFirstNDeliveries = count
}

func (s *TestWebhookServer) GetDeliveryCount(subscriptionID string) int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.receivedWebhooks[subscriptionID]
}

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	// Connect to the database
	dbConn, err := db.ConnectPostgres()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Connected to database successfully")

	if err := db.MigrateDB(dbConn); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("Database migrations applied successfully")

	// Setup Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_CONN_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
	})

	// Create context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the webhook worker scheduler with 5 workers
	worker.InitScheduler(ctx, 5, redisClient, dbConn)
	log.Println("Webhook worker scheduler initialized")

	// Create test webhook receiver server
	testServer := NewTestWebhookServer()
	testServer.SetFailCount(3) // Make it fail the first 3 attempts

	// Start test webhook server
	go func() {
		log.Println("Starting test webhook receiver server on :9090")
		if err := http.ListenAndServe(":9090", testServer); err != nil {
			log.Fatalf("Failed to start test webhook server: %v", err)
		}
	}()

	// Create test subscription
	subscription := types.Subscription{
		ID:        uuid.New(),
		Name:      "Test Subscription",
		TargetURL: "http://localhost:9090/webhook", // Point to our test server
		Secret:    "test-secret-123",
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	log.Printf("Created test subscription with ID: %s", subscription.ID.String())

	// Create test payload
	payload := map[string]interface{}{
		"event": "test_event",
		"data": map[string]interface{}{
			"id":        123,
			"timestamp": time.Now().Unix(),
			"details":   "This is a test webhook payload",
		},
	}

	payloadBytes, _ := json.Marshal(payload)

	// Start sending test webhooks
	var wg sync.WaitGroup

	// Test 1: Single webhook with retries
	wg.Add(1)
	go func() {
		defer wg.Done()

		log.Println("TEST 1: Single webhook with retries")

		// Process the webhook request
		taskID := worker.GlobalScheduler.ProcessRequest(
			subscription.ID.String(),
			subscription.TargetURL,
			subscription.Secret,
			json.RawMessage(payloadBytes),
		)

		log.Printf("Enqueued webhook task with ID: %s", taskID)

		// Poll the task status until it completes or fails
		for {
			time.Sleep(5 * time.Second)

			task, err := worker.GlobalScheduler.GetTaskStatus(taskID)
			if err != nil {
				log.Printf("Error getting task status: %v", err)
				continue
			}

			log.Printf("Task %s status: %s, attempts: %d/%d",
				taskID, task.Status, task.Attempts, task.MaxAttempts)

			if task.Status == "completed" || task.Status == "failed" {
				log.Printf("Task %s finished with status: %s", taskID, task.Status)
				break
			}
		}

		// Verify the webhook was delivered the expected number of times
		deliveryCount := testServer.GetDeliveryCount(taskID)
		log.Printf("Webhook was delivered %d times (expected: %d)",
			deliveryCount, testServer.failFirstNDeliveries+1)

		// Get logs for the task to verify they were created
		taskUUID, err := uuid.Parse(taskID)
		if err != nil {
			log.Printf("Error parsing task ID: %v", err)
		} else {
			// Query the database directly for logs
			query := `SELECT COUNT(*) FROM logs WHERE task_id = $1`
			var logCount int
			err = dbConn.QueryRow(query, taskUUID).Scan(&logCount)
			if err != nil {
				log.Printf("Error querying logs: %v", err)
			} else {
				log.Printf("Found %d log entries for task %s", logCount, taskID)
			}
		}
	}()

	// Test 2: Multiple concurrent webhooks
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Wait a bit to let the first test start
		time.Sleep(2 * time.Second)

		log.Println("TEST 2: Multiple concurrent webhooks")

		// Set test server to only fail the first attempt for these
		testServer.SetFailCount(1)

		// Send 10 webhook requests concurrently
		const numWebhooks = 10
		taskIDs := make([]string, numWebhooks)

		for i := 0; i < numWebhooks; i++ {
			// Create unique payload for each webhook
			webhookPayload := map[string]interface{}{
				"event": "test_event",
				"index": i,
				"data": map[string]interface{}{
					"id":        i,
					"timestamp": time.Now().Unix(),
					"details":   fmt.Sprintf("This is test webhook %d", i),
				},
			}

			payloadBytes, _ := json.Marshal(webhookPayload)

			// Process the webhook request
			taskIDs[i] = worker.GlobalScheduler.ProcessRequest(
				subscription.ID.String(),
				subscription.TargetURL,
				subscription.Secret,
				json.RawMessage(payloadBytes),
			)

			log.Printf("Enqueued webhook %d with task ID: %s", i, taskIDs[i])
		}

		// Wait for all webhooks to complete
		for i, taskID := range taskIDs {
			for {
				time.Sleep(2 * time.Second)

				task, err := worker.GlobalScheduler.GetTaskStatus(taskID)
				if err != nil {
					log.Printf("Error getting task %d status: %v", i, err)
					continue
				}

				if task.Status == "completed" || task.Status == "failed" {
					log.Printf("Task %d (%s) finished with status: %s after %d attempts",
						i, taskID, task.Status, task.Attempts)
					break
				}
			}
		}

		log.Println("All concurrent webhooks completed")

		// Query the database for logs associated with the subscription
		query := `SELECT COUNT(*) FROM logs WHERE subscription_id = $1`
		var logCount int
		err := dbConn.QueryRow(query, subscription.ID).Scan(&logCount)
		if err != nil {
			log.Printf("Error querying logs for subscription: %v", err)
		} else {
			log.Printf("Found %d log entries for subscription %s",
				logCount, subscription.ID.String())
		}
	}()

	// Test 3: Test task cancellation
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Wait for other tests to start
		time.Sleep(5 * time.Second)

		log.Println("TEST 3: Task cancellation")

		// Set test server to fail many times so we can cancel before completion
		testServer.SetFailCount(10)

		// Create payload
		cancellationPayload := map[string]interface{}{
			"event": "test_cancellation",
			"data": map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"details":   "This webhook should be cancelled",
			},
		}

		payloadBytes, _ := json.Marshal(cancellationPayload)

		// Process the webhook request
		taskID := worker.GlobalScheduler.ProcessRequest(
			subscription.ID.String(),
			subscription.TargetURL,
			subscription.Secret,
			json.RawMessage(payloadBytes),
		)

		log.Printf("Enqueued webhook task for cancellation with ID: %s", taskID)

		// Wait a bit to ensure the task has started processing
		time.Sleep(5 * time.Second)

		// Cancel the task
		err := worker.GlobalScheduler.CancelTask(taskID)
		if err != nil {
			log.Printf("Error cancelling task: %v", err)
		} else {
			log.Printf("Task %s cancelled successfully", taskID)
		}

		// Verify the task status
		time.Sleep(2 * time.Second)
		task, err := worker.GlobalScheduler.GetTaskStatus(taskID)
		if err != nil {
			log.Printf("Error getting cancelled task status: %v", err)
		} else {
			log.Printf("Cancelled task status: %s", task.Status)
		}
	}()

	// Test 4: Test log retention
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Wait for other tests to be well underway
		time.Sleep(10 * time.Second)

		log.Println("TEST 4: Log retention")

		// First, count current logs
		var initialCount int
		query := `SELECT COUNT(*) FROM logs`
		err := dbConn.QueryRow(query).Scan(&initialCount)
		if err != nil {
			log.Printf("Error counting logs: %v", err)
			return
		}

		log.Printf("Current log count: %d", initialCount)

		// Insert some old logs that should be cleaned up
		// These logs are set to be older than the retention period (72 hours)
		oldTime := time.Now().Add(-73 * time.Hour)

		// First, ensure we have the subscription in the database
		// We'll use the same subscription ID that was created for the test
		if subscription.ID == uuid.Nil {
			log.Println("Cannot run log retention test: no valid subscription available")
			return
		}

		insertQuery := `
			INSERT INTO logs 
			(id, task_id, subscription_id, target_url, timestamp, attempt_number, status, created_at, updated_at) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`

		// Insert 5 old log entries
		for i := 0; i < 5; i++ {
			_, err := dbConn.Exec(
				insertQuery,
				uuid.New(),             // id
				uuid.New(),             // task_id
				subscription.ID,        // subscription_id - use the test subscription that exists in the DB
				subscription.TargetURL, // target_url
				oldTime,                // timestamp
				i+1,                    // attempt_number
				"Success",              // status
				oldTime,                // created_at
				oldTime,                // updated_at
			)

			if err != nil {
				log.Printf("Error inserting old log: %v", err)
			}
		}

		// Count logs after insertion
		var afterInsertCount int
		err = dbConn.QueryRow(query).Scan(&afterInsertCount)
		if err != nil {
			log.Printf("Error counting logs after insertion: %v", err)
			return
		}

		log.Printf("Log count after inserting old logs: %d", afterInsertCount)

		// Force run of the log cleanup by manually executing it
		var logService = worker.GlobalScheduler.GetLogService()
		if logService == nil {
			log.Println("Unable to access log service")
			return
		}

		// Manually clean up logs older than 72 hours
		deletedCount, err := logService.CleanupOldLogs(context.Background(), 72)
		if err != nil {
			log.Printf("Error during manual log cleanup: %v", err)
		} else {
			log.Printf("Manually cleaned up %d old logs", deletedCount)
		}

		// Count logs after cleanup
		var afterCleanupCount int
		err = dbConn.QueryRow(query).Scan(&afterCleanupCount)
		if err != nil {
			log.Printf("Error counting logs after cleanup: %v", err)
			return
		}

		log.Printf("Log count after cleanup: %d", afterCleanupCount)
		log.Printf("Deleted %d logs during cleanup", afterInsertCount-afterCleanupCount)

		if afterInsertCount-afterCleanupCount >= 5 {
			log.Println("Log retention test passed: old logs were deleted")
		} else {
			log.Println("Log retention test failed: old logs were not deleted")
		}
	}()

	// Test 5: Test subscription caching feature
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Wait for other tests to be underway
		time.Sleep(15 * time.Second)

		log.Println("TEST 5: Subscription caching")

		// Step 1: Cache the subscription manually
		subscriptionKey := subscription.ID.String()
		err := cache.SetSubscriptionCache(redisClient, subscriptionKey, subscription)
		if err != nil {
			log.Printf("Error caching subscription: %v", err)
			return
		}
		log.Printf("Saved subscription to cache with ID: %s", subscriptionKey)

		// Step 2: Check that the subscription is in the cache
		cachedSub, err := cache.GetSubscriptionCache(redisClient, subscriptionKey)
		if err != nil {
			log.Printf("Error retrieving subscription from cache: %v", err)
			return
		}
		log.Printf("Successfully retrieved subscription from cache: %s", cachedSub.Name)

		// Step 3: Send a webhook using only the subscription ID
		// This will demonstrate using the cache to get target URL and secret
		webhookPayload := map[string]interface{}{
			"event": "caching_test",
			"data": map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"details":   "This webhook demonstrates subscription caching",
			},
		}

		payloadBytes, _ := json.Marshal(webhookPayload)

		// Process a webhook with only the subscription ID - it should get other details from cache
		log.Println("Sending webhook with only subscription ID - should use cache")
		taskID := worker.GlobalScheduler.ProcessRequest(
			subscriptionKey,
			"", // Empty target URL - should be filled from cache
			"", // Empty secret - should be filled from cache
			json.RawMessage(payloadBytes),
		)

		if taskID == "" {
			log.Println("Failed to create task with cached subscription details")
			return
		}

		log.Printf("Successfully created task using cached subscription: %s", taskID)

		// Wait for the task to complete
		for {
			time.Sleep(2 * time.Second)

			task, err := worker.GlobalScheduler.GetTaskStatus(taskID)
			if err != nil {
				log.Printf("Error getting task status: %v", err)
				continue
			}

			if task.Status == "completed" || task.Status == "failed" {
				log.Printf("Cached subscription task finished with status: %s, URL: %s",
					task.Status, task.TargetURL)
				break
			}
		}

		// Now verify that the target URL in the task was correctly populated from cache
		task, _ := worker.GlobalScheduler.GetTaskStatus(taskID)
		if task.TargetURL != subscription.TargetURL {
			log.Printf("Cache test failed: target URL mismatch. Expected %s, got %s",
				subscription.TargetURL, task.TargetURL)
		} else {
			log.Println("Cache test passed: target URL correctly retrieved from cache")
		}

		// Do a simple performance test
		start := time.Now()
		for i := 0; i < 100; i++ {
			worker.GlobalScheduler.GetSubscriptionDetails(subscriptionKey)
		}
		duration := time.Since(start)
		log.Printf("Time to fetch subscription 100 times from cache: %v", duration)
		log.Printf("Average time per fetch: %v", duration/100)
	}()

	// Create a channel to listen for termination signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Wait for all tests to complete or for termination signal
	go func() {
		wg.Wait()
		log.Println("All tests completed!")
		sigs <- syscall.SIGTERM // Signal termination
	}()

	// Wait for signal
	<-sigs
	log.Println("Shutting down...")

	// Cleanup
	worker.GlobalScheduler.Stop()
	log.Println("Test complete")
}
