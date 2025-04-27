Sure, here's the proposed content for the file: /internal/types/types.go

package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Task represents a delivery task
type Task struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SubscriptionID string             `bson:"subscription_id" json:"subscription_id"`
	Payload        json.RawMessage    `bson:"payload" json:"payload"`
	TargetURL      string             `bson:"target_url" json:"target_url"`
	Secret         string             `bson:"secret,omitempty" json:"secret,omitempty"`
	Attempts       int                `bson:"attempts" json:"attempts"`
	MaxAttempts    int                `bson:"max_attempts" json:"max_attempts"`
	LastAttempt    time.Time          `bson:"last_attempt,omitempty" json:"last_attempt,omitempty"`
	Status         string             `bson:"status" json:"status"`
	Response       string             `bson:"response,omitempty" json:"response,omitempty"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
	CompletedAt    time.Time          `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	NextAttemptAt  time.Time          `bson:"next_attempt_at,omitempty" json:"next_attempt_at,omitempty"` // When to try again
	WorkerID       int                `bson:"worker_id,omitempty" json:"worker_id,omitempty"`       // ID of worker processing this task
}

// TaskResult represents the result of a task execution
type TaskResult struct {
	Success     bool               `bson:"success" json:"success"`
	Message     string             `bson:"message" json:"message"`
	Task        *Task              `bson:"task" json:"task"`
	StatusCode  *int               `bson:"status_code,omitempty" json:"status_code,omitempty"`
	ErrorDetail string             `bson:"error_detail,omitempty" json:"error_detail,omitempty"`
}

// LogEntry represents a log entry for a delivery attempt
type LogEntry struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TaskID         primitive.ObjectID `bson:"task_id" json:"task_id"`
	SubscriptionID primitive.ObjectID `bson:"subscription_id" json:"subscription_id"`
	TargetURL      string             `bson:"target_url" json:"target_url"`
	Timestamp      time.Time          `bson:"timestamp" json:"timestamp"`
	AttemptNumber  int                `bson:"attempt_number" json:"attempt_number"`
	Status         string             `bson:"status" json:"status"`
	StatusCode     *int               `bson:"status_code,omitempty" json:"status_code,omitempty"`
	ErrorDetails   *string            `bson:"error_details,omitempty" json:"error_details,omitempty"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time          `bson:"updated_at" json:"updated_at"`
}