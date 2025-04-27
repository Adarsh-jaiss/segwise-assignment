CREATE TABLE IF NOT EXISTS logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL,
    subscription_id UUID NOT NULL,
    target_url TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    attempt_number INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL,
    status_code INTEGER,
    error_details TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (subscription_id) REFERENCES subscriptions(id)
);

-- Create an index on task_id and subscription_id for better query performance
CREATE INDEX idx_logs_task_id ON logs(task_id);
CREATE INDEX idx_logs_subscription_id ON logs(subscription_id);

-- Create a trigger to automatically update the updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_logs_updated_at
    BEFORE UPDATE ON logs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();