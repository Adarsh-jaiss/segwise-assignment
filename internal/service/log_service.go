package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/adarsh-jaiss/segwise/types"
	"github.com/google/uuid"
)

// LogService handles webhook delivery logs
type LogService struct {
	db *sql.DB
}

// NewLogService creates a new log service
func NewLogService(db *sql.DB) *LogService {
	return &LogService{
		db: db,
	}
}

// CreateLog creates a new delivery log entry
func (ls *LogService) CreateLog(ctx context.Context, log types.Logs) error {
	query := `
		INSERT INTO logs 
		(id, task_id, subscription_id, target_url, timestamp, attempt_number, status, status_code, error_details, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := ls.db.ExecContext(
		ctx,
		query,
		log.ID,
		log.TaskID,
		log.SubscriptionID,
		log.TargetURL,
		log.Timestamp,
		log.AttemptNumber,
		log.Status,
		log.StatusCode,
		log.ErrorDetails,
		log.CreatedAt,
		log.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create log: %v", err)
	}

	return nil
}

// GetLogsByTaskID retrieves all logs for a specific task
func (ls *LogService) GetLogsByTaskID(ctx context.Context, taskID uuid.UUID) ([]types.Logs, error) {
	query := `
		SELECT id, task_id, subscription_id, target_url, timestamp, attempt_number, 
		       status, status_code, error_details, created_at, updated_at
		FROM logs 
		WHERE task_id = $1
		ORDER BY timestamp DESC
	`

	rows, err := ls.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %v", err)
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
			return nil, fmt.Errorf("error scanning log row: %v", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// GetLogsBySubscriptionID retrieves all logs for a specific subscription
func (ls *LogService) GetLogsBySubscriptionID(ctx context.Context, subscriptionID uuid.UUID) ([]types.Logs, error) {
	query := `
		SELECT id, task_id, subscription_id, target_url, timestamp, attempt_number, 
		       status, status_code, error_details, created_at, updated_at
		FROM logs 
		WHERE subscription_id = $1
		ORDER BY timestamp DESC
	`

	rows, err := ls.db.QueryContext(ctx, query, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %v", err)
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
			return nil, fmt.Errorf("error scanning log row: %v", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// CleanupOldLogs deletes logs older than the specified retention period
func (ls *LogService) CleanupOldLogs(ctx context.Context, retentionHours int) (int64, error) {
	cutoffTime := time.Now().Add(-time.Duration(retentionHours) * time.Hour)

	query := `DELETE FROM logs WHERE created_at < $1`

	result, err := ls.db.ExecContext(ctx, query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old logs: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error getting rows affected: %v", err)
	}

	return rowsAffected, nil
}

// StartLogCleanupWorker starts a background worker to clean up old logs
func StartLogCleanupWorker(ctx context.Context, ls *LogService, retentionHours int, intervalHours int) {
	ticker := time.NewTicker(time.Duration(intervalHours) * time.Hour)
	go func() {
		// Run once immediately
		count, err := ls.CleanupOldLogs(ctx, retentionHours)
		if err != nil {
			log.Printf("Error cleaning up old logs: %v", err)
		} else {
			log.Printf("Cleaned up %d old logs", count)
		}

		// Then run on ticker
		for {
			select {
			case <-ticker.C:
				count, err := ls.CleanupOldLogs(ctx, retentionHours)
				if err != nil {
					log.Printf("Error cleaning up old logs: %v", err)
				} else {
					log.Printf("Cleaned up %d old logs", count)
				}
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
	log.Printf("Log cleanup worker started. Retention period: %d hours, Cleanup interval: %d hours", retentionHours, intervalHours)
}
