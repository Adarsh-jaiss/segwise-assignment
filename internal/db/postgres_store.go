package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/adarsh-jaiss/segwise/types"
	"github.com/google/uuid"
)

// CreateSubscriptionsInDB modification
func CreateSubscriptionsInDB(db *sql.DB, s types.Subscription) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database connection is nil")
	}

	query := `INSERT INTO subscriptions (id, name, target_url, payload, secret, active, created_at, updated_at)
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	result, err := db.Exec(query, s.ID, s.Name, s.TargetURL, s.Payload, s.Secret, s.Active, time.Now(), time.Now())
	if err != nil {
		return "", fmt.Errorf("failed to create subscriptions in table: %v", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("error getting rows affected: %v", err)
	}

	if rows == 0 {
		return "", fmt.Errorf("no rows were inserted")
	}

	return s.ID.String(), nil
}

// GetSubscriptionByIDFromStore modification
func GetSubscriptionByIDFromStore(db *sql.DB, id string) (*types.Subscription, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT id, name, target_url, payload, secret, active, created_at, updated_at 
			  FROM subscriptions WHERE id = $1`
	row := db.QueryRow(query, id)

	var subscription types.Subscription
	err := row.Scan(&subscription.ID, &subscription.Name, &subscription.TargetURL, 
					&subscription.Payload, &subscription.Secret, &subscription.Active, 
					&subscription.CreatedAt, &subscription.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("subscription not found")
	}
	if err != nil {
		return nil, fmt.Errorf("error scanning subscription: %v", err)
	}

	return &subscription, nil
}

// UpdateSubscriptionInDB modification
func UpdateSubscriptionInDB(db *sql.DB, s types.Subscription) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	setClause := ""
	args := make([]interface{}, 0)
	argPosition := 1

	if s.Name != "" {
		setClause += fmt.Sprintf("name = $%d, ", argPosition)
		args = append(args, s.Name)
		argPosition++
	}
	if s.TargetURL != "" {
		setClause += fmt.Sprintf("target_url = $%d, ", argPosition)
		args = append(args, s.TargetURL)
		argPosition++
	}
	if s.Payload != nil {
		setClause += fmt.Sprintf("payload = $%d, ", argPosition)
		args = append(args, s.Payload)
		argPosition++
	}
	if s.Secret != "" {
		setClause += fmt.Sprintf("secret = $%d, ", argPosition)
		args = append(args, s.Secret)
		argPosition++
	}
	
	setClause += fmt.Sprintf("active = $%d, ", argPosition)
	args = append(args, s.Active)
	argPosition++

	setClause += fmt.Sprintf("updated_at = $%d ", argPosition)
	args = append(args, time.Now())
	argPosition++

	query := fmt.Sprintf("UPDATE subscriptions SET %s WHERE id = $%d", setClause, argPosition)
	args = append(args, s.ID)

	result, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update subscription: %v", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %v", err)
	}

	if rows == 0 {
		return fmt.Errorf("no rows were updated")
	}

	return nil
}

func DeleteSubscriptionByIDFromStore(db *sql.DB, id string) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// First delete all logs associated with the subscription
	_, err = tx.Exec("DELETE FROM logs WHERE subscription_id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete associated logs: %v", err)
	}

	result, err := tx.Exec("DELETE FROM subscriptions WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %v", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %v", err)
	}

	if rows == 0 {
		return fmt.Errorf("no subscription found with id %s", id)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func GetAllSubscriptionsFromDB(db *sql.DB) ([]types.Subscription, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	rows, err := db.Query("SELECT id, name, target_url, secret, payload, created_at, updated_at FROM subscriptions")
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions: %v", err)
	}
	defer rows.Close()

	var subscriptions []types.Subscription
	for rows.Next() {
		var subscription types.Subscription
		err := rows.Scan(
			&subscription.ID,
			&subscription.Name,
			&subscription.TargetURL,
			&subscription.Secret,
			&subscription.Payload,
			&subscription.CreatedAt,
			&subscription.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning subscription row: %v", err)
		}
		subscriptions = append(subscriptions, subscription)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating subscription rows: %v", err)
	}

	return subscriptions, nil
}


func GetRecentDeliveryAttemptsFromDB(db *sql.DB, subscriptionUUID uuid.UUID, limit int) ([]types.Logs, error) {
	query := `
		SELECT id, task_id, subscription_id, target_url, timestamp, attempt_number, 
			status, status_code, error_details, created_at, updated_at
		FROM logs 
		WHERE subscription_id = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := db.Query(query, subscriptionUUID, limit)
	if err != nil {
		return nil, fmt.Errorf("Failed to query logs: %v", err)
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
			return nil, fmt.Errorf("Error scanning log row: %v", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}
