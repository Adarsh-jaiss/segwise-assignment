package db

import (
	"database/sql"
	"fmt"
	"time"
	"github.com/adarsh-jaiss/segwise/types"
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

	query := `DELETE FROM subscriptions WHERE id = $1`
	result, err := db.Exec(query, id)
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

	return nil
}