package types

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Subscription represents a webhook subscription
type Subscription struct {
	ID        uuid.UUID       `json:"id" db:"id"`
	Name      string          `json:"name" db:"name"`
	TargetURL string          `json:"target_url" db:"target_url"`
	Payload   json.RawMessage `json:"payload"`
	Secret    string          `json:"secret,omitempty" db:"secret"`
	Active    bool            `json:"active" db:"active"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

type Logs struct {
	ID             uuid.UUID `db:"id" json:"id"`
	TaskID         uuid.UUID `db:"task_id" json:"task_id"`
	SubscriptionID uuid.UUID `db:"subscription_id" json:"subscription_id"`
	TargetURL      string    `db:"target_url" json:"target_url"`
	Timestamp      time.Time `db:"timestamp" json:"timestamp"`
	AttemptNumber  int       `db:"attempt_number" json:"attempt_number"`
	Status         string    `db:"status" json:"status"`
	StatusCode     *int      `db:"status_code,omitempty" json:"status_code,omitempty"`
	ErrorDetails   *string   `db:"error_details,omitempty" json:"error_details,omitempty"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}
