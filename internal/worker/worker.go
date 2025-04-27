// worker.go
package worker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/adarsh-jaiss/segwise/internal/cache"
	"github.com/adarsh-jaiss/segwise/internal/service"
	"github.com/adarsh-jaiss/segwise/types"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Task represents a delivery task
type Task struct {
	ID             string          `json:"id"`
	SubscriptionID string          `json:"subscription_id"`
	Payload        json.RawMessage `json:"payload"`
	TargetURL      string          `json:"target_url"`
	Secret         string          `json:"secret,omitempty"`
	Attempts       int             `json:"attempts"`
	MaxAttempts    int             `json:"max_attempts"`
	LastAttempt    time.Time       `json:"last_attempt,omitempty"`
	Status         string          `json:"status"`
	Response       string          `json:"response,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	CompletedAt    time.Time       `json:"completed_at,omitempty"`
	NextAttemptAt  time.Time       `json:"next_attempt_at,omitempty"` // When to try again
	WorkerID       int             `json:"worker_id,omitempty"`       // ID of worker processing this task
}

// TaskResult represents the result of a task execution
type TaskResult struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	Task        *Task  `json:"task"`
	StatusCode  *int   `json:"status_code,omitempty"`
	ErrorDetail string `json:"error_detail,omitempty"`
}

// Scheduler manages the task queue and workers
type Scheduler struct {
	taskQueue   chan *Task
	workerWg    sync.WaitGroup
	numWorkers  int
	redis       *redis.Client
	ctx         context.Context
	runningChan chan struct{} // Signal channel to indicate the scheduler is running
	db          *sql.DB       // Database connection for logs and subscriptions
	logService  *service.LogService
}

// NewScheduler creates a new scheduler with the specified number of workers
func NewScheduler(ctx context.Context, numWorkers int, redis *redis.Client, db *sql.DB) *Scheduler {
	logService := service.NewLogService(db)
	return &Scheduler{
		taskQueue:   make(chan *Task, 100), // Buffer for 100 tasks
		numWorkers:  numWorkers,
		redis:       redis,
		ctx:         ctx,
		runningChan: make(chan struct{}),
		db:          db,
		logService:  logService,
	}
}

// Start initializes the worker goroutines and task scheduler
func (s *Scheduler) Start() {
	// Start the task dispatcher
	go s.taskDispatcher()

	// Start worker goroutines
	for i := 0; i < s.numWorkers; i++ {
		s.workerWg.Add(1)
		workerID := i + 1
		go s.worker(workerID)
	}
	log.Printf("Started %d webhook delivery workers", s.numWorkers)

	// Start a goroutine to recover tasks from Redis that were being processed
	// when the service was shut down
	go s.recoverPendingTasks()

	// Signal that the scheduler is running
	close(s.runningChan)
}

// taskDispatcher is responsible for polling Redis for tasks that are ready to be retried
func (s *Scheduler) taskDispatcher() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.dispatchScheduledTasks()
		}
	}
}

// dispatchScheduledTasks finds tasks in Redis that are scheduled for retry and are due
func (s *Scheduler) dispatchScheduledTasks() {
	now := time.Now()

	// Get all tasks with status "retry_pending"
	retryKey := "webhook:tasks:retry_pending"
	taskIDs, err := s.redis.SMembers(s.ctx, retryKey).Result()
	if err != nil {
		log.Printf("Error getting retry pending tasks: %v", err)
		return
	}

	for _, taskID := range taskIDs {
		task, err := s.GetTaskStatus(taskID)
		if err != nil {
			log.Printf("Error retrieving task %s: %v", taskID, err)
			continue
		}

		// Check if it's time to retry this task
		if task.NextAttemptAt.Before(now) {
			// Try to acquire a lock on this task
			if s.acquireTaskLock(task.ID) {
				// Update status to queued and remove from retry set
				task.Status = "queued"
				task.WorkerID = 0 // Clear worker assignment
				s.updateTaskInRedis(task)
				s.redis.SRem(s.ctx, retryKey, task.ID)

				// Send to worker queue
				select {
				case s.taskQueue <- task:
					log.Printf("Dispatched task %s for retry", task.ID)
				default:
					// If queue is full, put it back in retry set
					task.Status = "retry_pending"
					s.updateTaskInRedis(task)
					s.redis.SAdd(s.ctx, retryKey, task.ID)
					s.releaseTaskLock(task.ID)
					log.Printf("Task queue full, task %s re-scheduled", task.ID)
				}
			}
		}
	}
}

// recoverPendingTasks finds tasks that were left in "in_progress" status
// and resets them to be retried
func (s *Scheduler) recoverPendingTasks() {
	// Wait for scheduler to be fully started
	<-s.runningChan

	log.Println("Recovering pending tasks from previous runs...")

	// Find all in-progress tasks
	inProgressKey := "webhook:tasks:in_progress"
	taskIDs, err := s.redis.SMembers(s.ctx, inProgressKey).Result()
	if err != nil {
		log.Printf("Error retrieving in-progress tasks: %v", err)
		return
	}

	for _, taskID := range taskIDs {
		task, err := s.GetTaskStatus(taskID)
		if err != nil {
			log.Printf("Error retrieving task %s: %v", taskID, err)
			continue
		}

		// Reset task for retry
		task.Status = "retry_pending"
		task.NextAttemptAt = time.Now().Add(10 * time.Second) // Try again in 10 seconds
		task.WorkerID = 0

		s.updateTaskInRedis(task)
		s.redis.SRem(s.ctx, inProgressKey, task.ID)
		s.redis.SAdd(s.ctx, "webhook:tasks:retry_pending", task.ID)

		log.Printf("Recovered task %s for retry", task.ID)
	}
}

// Stop waits for all workers to finish
func (s *Scheduler) Stop() {
	// Signal context cancellation
	close(s.taskQueue)
	s.workerWg.Wait()
	log.Println("All webhook delivery workers stopped")
}

// EnqueueTask adds a task to the queue and returns immediately
func (s *Scheduler) EnqueueTask(task *Task) {
	// Store task in Redis for persistence
	task.Status = "queued"
	s.saveTaskToRedis(task)

	// Send to channel for processing if possible
	select {
	case s.taskQueue <- task:
		// Task was added to channel
	default:
		// Channel is full, update status to retry later
		task.Status = "retry_pending"
		task.NextAttemptAt = time.Now().Add(10 * time.Second)
		s.updateTaskInRedis(task)
		s.redis.SAdd(s.ctx, "webhook:tasks:retry_pending", task.ID)
		log.Printf("Task queue full, task %s scheduled for later", task.ID)
	}
}

// GetSubscriptionDetails retrieves subscription details from cache or database
func (s *Scheduler) GetSubscriptionDetails(subscriptionID string) (*types.Subscription, error) {
	// Try to get from cache first
	sub, err := cache.GetSubscriptionCache(s.redis, subscriptionID)
	if err == nil {
		// Cache hit
		return &sub, nil
	}

	// Cache miss, fetch from database
	subscription, err := service.FetchAndCacheSubscription(s.db, s.redis, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription details: %v", err)
	}

	// Return the fetched subscription
	return &subscription, nil
}

// ProcessRequest creates a task from the webhook request and enqueues it
func (s *Scheduler) ProcessRequest(subscriptionID string, targetURL string, secret string, payload json.RawMessage) string {
	// Create a unique task ID
	taskID := fmt.Sprintf("task_%s_%d", subscriptionID, time.Now().UnixNano())

	// If targetURL and secret are empty, fetch them from subscription cache or database
	if targetURL == "" || secret == "" {
		subscription, err := s.GetSubscriptionDetails(subscriptionID)
		if err != nil {
			log.Printf("Error fetching subscription details: %v", err)
			return ""
		}
		targetURL = subscription.TargetURL
		secret = subscription.Secret
	}

	// Create the task
	task := &Task{
		ID:             taskID,
		SubscriptionID: subscriptionID,
		Payload:        payload,
		TargetURL:      targetURL,
		Secret:         secret,
		Attempts:       0,
		MaxAttempts:    5,
		Status:         "new",
		CreatedAt:      time.Now(),
	}

	// Enqueue the task for processing
	s.EnqueueTask(task)

	return taskID
}

// saveTaskToRedis stores the task in Redis for persistence
func (s *Scheduler) saveTaskToRedis(task *Task) error {
	taskKey := fmt.Sprintf("webhook:task:%s", task.ID)
	taskJSON, err := json.Marshal(task)
	if err != nil {
		log.Printf("Error marshaling task %s: %v", task.ID, err)
		return err
	}

	err = s.redis.Set(s.ctx, taskKey, taskJSON, 48*time.Hour).Err()
	if err != nil {
		log.Printf("Error saving task %s to Redis: %v", task.ID, err)
		return err
	}

	// Also add to a list of tasks for this subscription
	subscriptionTasksKey := fmt.Sprintf("webhook:subscription:%s:tasks", task.SubscriptionID)
	err = s.redis.RPush(s.ctx, subscriptionTasksKey, task.ID).Err()
	if err != nil {
		log.Printf("Error adding task %s to subscription task list: %v", task.ID, err)
	}

	// Add to status set based on current status
	statusKey := fmt.Sprintf("webhook:tasks:%s", task.Status)
	s.redis.SAdd(s.ctx, statusKey, task.ID)

	return nil
}

// updateTaskInRedis updates an existing task in Redis
func (s *Scheduler) updateTaskInRedis(task *Task) error {
	// First get the current task to know its status
	oldTask, err := s.GetTaskStatus(task.ID)
	oldStatus := "unknown"
	if err == nil {
		oldStatus = oldTask.Status
	}

	// Now update the task
	taskKey := fmt.Sprintf("webhook:task:%s", task.ID)
	taskJSON, err := json.Marshal(task)
	if err != nil {
		log.Printf("Error marshaling task %s: %v", task.ID, err)
		return err
	}

	err = s.redis.Set(s.ctx, taskKey, taskJSON, 48*time.Hour).Err()
	if err != nil {
		log.Printf("Error updating task %s in Redis: %v", task.ID, err)
		return err
	}

	// Update status sets if status changed
	if oldStatus != task.Status {
		// Remove from old status set
		if oldStatus != "unknown" {
			oldStatusKey := fmt.Sprintf("webhook:tasks:%s", oldStatus)
			s.redis.SRem(s.ctx, oldStatusKey, task.ID)
		}

		// Add to new status set
		newStatusKey := fmt.Sprintf("webhook:tasks:%s", task.Status)
		s.redis.SAdd(s.ctx, newStatusKey, task.ID)
	}

	return nil
}

// acquireTaskLock attempts to lock a task for processing
// Returns true if successful, false if already locked
func (s *Scheduler) acquireTaskLock(taskID string) bool {
	lockKey := fmt.Sprintf("webhook:task:%s:lock", taskID)
	// Try to set the lock with expiration (30 min to handle long-running tasks)
	success, err := s.redis.SetNX(s.ctx, lockKey, "1", 30*time.Minute).Result()
	if err != nil {
		log.Printf("Error acquiring lock for task %s: %v", taskID, err)
		return false
	}
	return success
}

// releaseTaskLock releases a task lock
func (s *Scheduler) releaseTaskLock(taskID string) {
	lockKey := fmt.Sprintf("webhook:task:%s:lock", taskID)
	s.redis.Del(s.ctx, lockKey)
}

// worker processes tasks from the queue
func (s *Scheduler) worker(id int) {
	defer s.workerWg.Done()
	log.Printf("Worker %d started", id)

	for task := range s.taskQueue {
		// Double-check we can acquire the lock
		if !s.acquireTaskLock(task.ID) {
			log.Printf("Worker %d skipping task %s - already locked by another worker", id, task.ID)
			continue
		}

		// Update task status to in_progress and assign worker ID
		task.Status = "in_progress"
		task.WorkerID = id
		s.updateTaskInRedis(task)

		log.Printf("Worker %d processing task %s for subscription %s", id, task.ID, task.SubscriptionID)

		// Process the task
		result := s.processTask(task)

		// Create a log entry for this delivery attempt
		s.logDeliveryAttempt(task, result)

		// Release lock and update status
		if result.Success {
			// Task completed successfully
			result.Task.Status = "completed"
			result.Task.CompletedAt = time.Now()
			s.updateTaskInRedis(result.Task)
		} else if result.Task.Status == "failed" {
			// Task failed permanently
			s.updateTaskInRedis(result.Task)
		} else {
			// Task needs retry
			result.Task.Status = "retry_pending"
			backoffSeconds := calculateBackoff(result.Task.Attempts)
			result.Task.NextAttemptAt = time.Now().Add(time.Duration(backoffSeconds) * time.Second)
			s.updateTaskInRedis(result.Task)

			// Add to retry set
			s.redis.SAdd(s.ctx, "webhook:tasks:retry_pending", result.Task.ID)
			log.Printf("Task %s scheduled for retry in %d seconds", result.Task.ID, backoffSeconds)
		}

		// Release the lock
		s.releaseTaskLock(task.ID)
	}
}

// logDeliveryAttempt creates a log entry for a delivery attempt
func (s *Scheduler) logDeliveryAttempt(task *Task, result TaskResult) {
	// Create a deterministic UUID for the task ID instead of trying to parse the string
	// This avoids the "invalid UUID length" error
	taskIDStr := task.ID
	taskID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(taskIDStr))

	// First, check if the subscription actually exists in the database
	// to avoid foreign key constraint violations
	_, err := s.GetSubscriptionDetails(task.SubscriptionID)
	if err != nil {
		log.Printf("Cannot log delivery attempt: subscription %s not found: %v", task.SubscriptionID, err)
		return // Skip logging if subscription doesn't exist
	}

	// Convert subscription ID string to UUID
	subscriptionID, err := uuid.Parse(task.SubscriptionID)
	if err != nil {
		log.Printf("Error parsing subscription ID to UUID: %v", err)
		// We've already verified the subscription exists, so we shouldn't have parsing errors
		// But if we do, we should skip logging rather than creating an invalid entry
		return
	}

	// Determine status for the log
	var status string
	if result.Success {
		status = "Success"
	} else if task.Status == "failed" {
		status = "Failure"
	} else {
		status = "Failed Attempt"
	}

	// Create log entry
	logEntry := types.Logs{
		ID:             uuid.New(),
		TaskID:         taskID,
		SubscriptionID: subscriptionID,
		TargetURL:      task.TargetURL,
		Timestamp:      task.LastAttempt,
		AttemptNumber:  task.Attempts,
		Status:         status,
		StatusCode:     result.StatusCode,
		ErrorDetails:   &result.ErrorDetail,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Save to database
	if err := s.logService.CreateLog(s.ctx, logEntry); err != nil {
		log.Printf("Error logging delivery attempt: %v", err)
	}
}

// processTask attempts to deliver the payload and implements retry logic
func (s *Scheduler) processTask(task *Task) TaskResult {
	// Increment attempt counter
	task.Attempts++
	task.LastAttempt = time.Now()

	// Update task in Redis
	s.updateTaskInRedis(task)

	success, response, statusCode, errorDetail := deliverPayload(task)
	log.Printf("Received response for task %s: %s", task.ID, response)
	task.Response = response

	result := TaskResult{
		Success:     success,
		Task:        task,
		StatusCode:  statusCode,
		ErrorDetail: errorDetail,
	}

	if success {
		result.Message = fmt.Sprintf("Task %s delivered successfully after %d attempts", task.ID, task.Attempts)
		return result
	}

	// Check if we've reached max attempts
	if task.Attempts >= task.MaxAttempts {
		task.Status = "failed"
		result.Message = fmt.Sprintf("Task %s failed after %d attempts", task.ID, task.Attempts)
		return result
	}

	// Task will be retried
	result.Message = fmt.Sprintf("Task %s delivery failed, attempt %d/%d. Will retry.",
		task.ID, task.Attempts, task.MaxAttempts)
	return result
}

// calculateBackoff implements exponential backoff strategy
func calculateBackoff(attempt int) int {
	// Base values: 10s, 30s, 1m, 5m, 15m
	backoffTimes := []int{10, 30, 60, 300, 900}

	if attempt <= len(backoffTimes) {
		return backoffTimes[attempt-1]
	}

	// For any attempts beyond our predefined values, use exponential backoff with jitter
	base := math.Min(float64(900), math.Pow(2, float64(attempt))*5)
	return int(base)
}

// deliverPayload sends the HTTP request to the target URL
func deliverPayload(task *Task) (bool, string, *int, string) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("POST", task.TargetURL, bytes.NewBuffer(task.Payload))
	if err != nil {
		return false, fmt.Sprintf("Error creating request: %v", err), nil, err.Error()
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-ID", task.ID)
	req.Header.Set("X-Webhook-Attempt", fmt.Sprintf("%d", task.Attempts))

	// If a secret is provided, implement signature verification
	if task.Secret != "" {
		// Example: Add a simple header-based authentication
		req.Header.Set("X-Webhook-Secret", task.Secret)
		// In a real implementation, you'd typically use the secret to generate and verify
		// a signature of the payload
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("HTTP request failed: %v", err), nil, err.Error()
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode

	// Check if response is successful (2xx)
	if statusCode >= 200 && statusCode < 300 {
		return true, fmt.Sprintf("Successful delivery, status: %d", statusCode), &statusCode, ""
	}

	errorDetail := fmt.Sprintf("HTTP request failed with status code: %d", statusCode)
	return false, fmt.Sprintf("Delivery failed with status: %d", statusCode), &statusCode, errorDetail
}

// GetTaskStatus retrieves a task's status from Redis
func (s *Scheduler) GetTaskStatus(taskID string) (*Task, error) {
	taskKey := fmt.Sprintf("webhook:task:%s", taskID)
	taskJSON, err := s.redis.Get(s.ctx, taskKey).Result()
	if err != nil {
		return nil, err
	}

	var task Task
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		return nil, err
	}

	return &task, nil
}

// GetSubscriptionTasks retrieves all tasks for a subscription
func (s *Scheduler) GetSubscriptionTasks(subscriptionID string) ([]*Task, error) {
	subscriptionTasksKey := fmt.Sprintf("webhook:subscription:%s:tasks", subscriptionID)
	taskIDs, err := s.redis.LRange(s.ctx, subscriptionTasksKey, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	var tasks []*Task
	for _, taskID := range taskIDs {
		task, err := s.GetTaskStatus(taskID)
		if err == nil {
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

// CancelTask cancels a pending task if it hasn't been completed yet
func (s *Scheduler) CancelTask(taskID string) error {
	// First, try to acquire lock to prevent race conditions
	if !s.acquireTaskLock(taskID) {
		return fmt.Errorf("task is currently being processed, cannot cancel")
	}
	defer s.releaseTaskLock(taskID)

	task, err := s.GetTaskStatus(taskID)
	if err != nil {
		return err
	}

	// Only cancel if task is not yet completed or failed
	if task.Status != "completed" && task.Status != "failed" && task.Status != "cancelled" {
		oldStatus := task.Status
		task.Status = "cancelled"

		// Update task in Redis
		if err := s.updateTaskInRedis(task); err != nil {
			return err
		}

		// Remove from status sets
		oldStatusKey := fmt.Sprintf("webhook:tasks:%s", oldStatus)
		s.redis.SRem(s.ctx, oldStatusKey, task.ID)

		// Add to cancelled set
		s.redis.SAdd(s.ctx, "webhook:tasks:cancelled", task.ID)

		return nil
	}

	return fmt.Errorf("task is in %s state and cannot be cancelled", task.Status)
}

// GlobalScheduler is a singleton instance of the scheduler
var GlobalScheduler *Scheduler

// InitScheduler initializes the global scheduler instance
func InitScheduler(ctx context.Context, numWorkers int, redis *redis.Client, db *sql.DB) {
	GlobalScheduler = NewScheduler(ctx, numWorkers, redis, db)
	GlobalScheduler.Start()

	// Start log retention worker - clean logs older than 72 hours every 6 hours
	service.StartLogCleanupWorker(ctx, GlobalScheduler.logService, 72, 6)
}

// GetLogService returns the log service instance
func (s *Scheduler) GetLogService() *service.LogService {
	return s.logService
}
