# Segwise - Webhook Delivery System

Segwise is a robust, scalable webhook delivery system designed to reliably deliver webhook payloads to target endpoints with retry capability, delivery tracking, and comprehensive analytics.

## Live Deployment

The application is deployed and accessible at:
[https://segwise-webhooks.example.com](https://segwise-webhooks.example.com)

## Architecture Overview

### Technology Stack

- **Framework**: Go with Echo web framework - chosen for its high performance, simplicity, and middleware support
- **Database**: PostgreSQL - selected for its reliability, ACID compliance, and robust feature set
- **Caching**: Redis - used for fast in-memory storage of task status and subscription details
- **Async Task/Queueing System**: Custom in-memory queue with Redis persistence - provides reliable task processing with recovery capabilities
- **Retry Strategy**: Exponential backoff with jitter - reduces server load while efficiently retrying failed webhook deliveries

### Key Components

1. **API Server**: Handles subscription management and webhook ingestion
2. **Background Workers**: Process queued webhook deliveries with automatic retries
3. **Database**: Stores subscription data and delivery logs
4. **Redis Cache**: Maintains task status and improves performance

## Database Schema and Indexing

### Tables

1. **Subscriptions**
   ```sql
   CREATE TABLE subscriptions (
       id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
       name TEXT NOT NULL,
       target_url TEXT NOT NULL,
       secret TEXT,
       active BOOLEAN NOT NULL DEFAULT true,
       created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
       updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
   );
   ```

2. **Logs**
   ```sql
   CREATE TABLE logs (
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
   ```

### Indexes

```sql
CREATE INDEX idx_logs_task_id ON logs(task_id);
CREATE INDEX idx_logs_subscription_id ON logs(subscription_id);
```

These indexes optimize queries for:
- Finding all logs for a specific task
- Retrieving all delivery attempts for a subscription
- Analyzing delivery performance

## Setup Instructions

### Prerequisites

- Docker and Docker Compose
- Git

### Running Locally with Docker

1. **Clone the repository**
   ```bash
   git clone https://github.com/yourusername/segwise.git
   cd segwise
   ```

2. **Run the application using Docker Compose**
   ```bash
   docker-compose up -d
   ```
   This will:
   - Build the application container
   - Start PostgreSQL and Redis containers
   - Apply database migrations automatically
   - Start the webhook delivery workers

3. **Verify the application is running**
   ```bash
   curl http://localhost:8080/
   ```
   You should see "Hello, World!" as the response

4. **Run the test script to verify functionality**
   ```bash
   ./test.sh
   ```
   This will create a test subscription, ingest a webhook, and verify delivery

### Stopping the Application

```bash
docker-compose down
```

To remove all data volumes:
```bash
docker-compose down -v
```

## API Documentation

### Subscription Management

#### Create a Subscription

```bash
curl -X POST http://localhost:8080/subscriptions \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Test Webhook",
    "target_url": "https://jsonplaceholder.typicode.com/posts",
    "secret": "my-signing-secret",
    "payload": {
      "custom_field": "value",
      "another_field": 123
    }
  }'
```

#### Get a Subscription

```bash
curl -X GET http://localhost:8080/subscriptions/YOUR_SUBSCRIPTION_ID
```

#### Update a Subscription

```bash
curl -X PATCH http://localhost:8080/subscriptions/YOUR_SUBSCRIPTION_ID \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated Webhook Name",
    "target_url": "https://jsonplaceholder.typicode.com/posts",
    "payload": {
      "custom_field": "new_value",
      "another_field": 456
    }
  }'
```

#### Delete a Subscription

```bash
curl -X DELETE http://localhost:8080/subscriptions/YOUR_SUBSCRIPTION_ID
```

### Webhook Delivery

#### Ingest a Webhook

```bash
curl -X POST http://localhost:8080/api/ingest/YOUR_SUBSCRIPTION_ID \
  -H "Content-Type: application/json" \
  -d '{
    "title": "foo",
    "body": "bar",
    "userId": 1
  }'
```

#### Check Task Status

```bash
curl -X GET http://localhost:8080/api/tasks/YOUR_TASK_ID
```

#### Get Subscription Tasks

```bash
curl -X GET http://localhost:8080/api/subscriptions/YOUR_SUBSCRIPTION_ID/tasks
```

### Analytics and Logs

#### Get Task Logs

```bash
curl -X GET http://localhost:8080/api/tasks/YOUR_TASK_ID/logs
```

#### Get Subscription Logs

```bash
curl -X GET http://localhost:8080/api/subscriptions/YOUR_SUBSCRIPTION_ID/logs
```

#### Get Task Analytics

```bash
curl -X GET http://localhost:8080/api/analytics/tasks/YOUR_TASK_ID
```

#### Get Recent Delivery Attempts

```bash
curl -X GET http://localhost:8080/api/analytics/subscriptions/YOUR_SUBSCRIPTION_ID/recent-attempts?limit=10
```

## Testing with JSONPlaceholder

For testing purposes, we recommend using [JSONPlaceholder](https://jsonplaceholder.typicode.com), a free online REST API that can be used as a webhook target.

Example webhook configuration:
- Target URL: https://jsonplaceholder.typicode.com/posts
- Payload Format: 
  ```json
  {
    "title": "foo",
    "body": "bar",
    "userId": 1
  }
  ```

JSONPlaceholder will return a mock response with status code 201 (Created), making it ideal for testing webhook delivery.

## Cost Estimation

### Assumptions
- 5,000 webhooks ingested daily (150,000 monthly)
- Average 1.2 delivery attempts per webhook
- 24/7 operation

### AWS Free Tier Estimation

| Service | Usage | Free Tier Limit | Monthly Cost |
|---------|-------|-----------------|--------------|
| EC2 (t2.micro) | 1 instance, 24/7 | 750 hours | $0 (within free tier) |
| RDS PostgreSQL | 20 GB storage | 20 GB, 750 hours | $0 (within free tier) |
| ElastiCache Redis | 1 node, cache.t2.micro | Not in free tier | ~$12.50 |
| Network Transfer | ~10 GB/month | 1 GB free | ~$0.90 |
| **Total** | | | **~$13.40** |

Notes:
- Cost would increase beyond free tier usage
- Additional monitoring, load balancing, or data backup services would increase costs
- Alternative: Heroku Hobby tier (~$7 for dyno + ~$7 for Postgres + ~$15 for Redis) = ~$29/month

## Assumptions and Limitations

1. **Webhook Payload Size**: Maximum payload size is 1MB
2. **Rate Limiting**: No rate limiting implemented in the current version
3. **Authentication**: Basic authentication for API endpoints not implemented
4. **Idempotency**: Webhooks are not guaranteed to be delivered exactly once
5. **Disaster Recovery**: Basic recovery from application crashes, but no cross-region redundancy
6. **Monitoring**: No built-in monitoring or alerting system

## Future Improvements

1. Implement API authentication
2. Add rate limiting for webhook ingestion
3. Support webhook signatures for payload verification
4. Add monitoring and alerting
5. Implement event filtering capabilities
6. Add support for batch processing of webhooks
