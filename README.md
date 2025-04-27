# Segwise - Webhook Delivery System

Segwise is a robust, scalable webhook delivery system designed to reliably deliver webhook payloads to target endpoints with retry capability, delivery tracking, and comprehensive analytics.

## Live Deployment

The application is deployed and accessible at:
[https://segwise-assignment-ui.onrender.com/](https://segwise-assignment-ui.onrender.com/)

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
       payload JSONB,
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

### Running Locally with and without Docker

1. **Clone the repository**
   ```bash
   git clone https://github.com/Adarsh-jaiss/segwise-assignment.git
   cd segwise-assingment
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
  
   ```

2. **Run the application without Docker**

  Add the following environment variables to your `.env` file:
  ```
  DB_USER= postgres
  DB_PASSWORD= postgres
  DB_HOST= localhost
  DB_PORT = 5432
  DB_NAME = segwise
  REDIS_CONN_ADDR = localhost:6379
  REDIS_PASSWORD =
  REDDIS_USERNAME=
  ```

  and make sure you have PostgreSQL and Redis installed locally.

   Then run the following commands:

   ```bash
   go mod download
   make run
   ```
   This will:
   - Download all Go dependencies
   - Start the application using the Makefile
   - Connect to PostgreSQL and Redis
   - Apply database migrations
   - Start the webhook delivery workers

3. **Verify the application is running**
   ```bash
   curl http://localhost:8080/
   ```

   and you can also open the UI in your browser using this command in linux/macOS:
    ```bash
    open ui/index.html
    ```
    or in windows:
      ```bash
      start ui/index.html
      ```
   This will open the UI in your default web browser.
   You should see "Hello, World!" as the response

4. **Create a test subscription**
   ```bash
   curl -X POST http://localhost:8080/api/subscriptions \
    -H "Content-Type: application/json" \
    -d '{
      "name": "test-1",
      "target_url": "https://jsonplaceholder.typicode.com/posts",
      "payload": {
        "title": "foo",
        "body": "bar",
        "userId": 1
      }
    }'
   ```

   This will:
   - Create a new subscription in the database
   - Generate a unique UUID for the subscription
   - Set the subscription as active by default
   - Queue the initial webhook delivery task
   - Return a response with the subscription ID and confirmation message
   - The webhook will be delivered to the specified target URL (JSONPlaceholder in this example)
  
5. **Test the webhook delivery system and all API's**
   ```bash
   # Run the test webhook script to test worker implementation
   go run internal/test/test-webhook.go

   # Run the test script to test all API endpoints
   ./test.sh
   ```

   The test script will:
   - Create test subscriptions
   - Send test webhooks
   - Verify delivery status
   - Check logs and analytics

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

Creates a new webhook subscription and returns the subscription ID.

- **URL**: `/api/subscriptions`
- **Method**: `POST`
- **Content-Type**: `application/json`
- **Request Body**:
  ```json
  {
    "name": "test-1",
    "target_url": "https://jsonplaceholder.typicode.com/posts",
    "payload": {
      "title": "foo",
      "body": "bar",
      "userId": 1
    }
  }
  ```
- **Success Response**:
  - **Code**: `202 Accepted`
  - **Content**:
    ```json
    {
      "id": "9332827c-7972-454f-bee1-a128047c5557",
      "message": "Subscription created"
    }
    ```
- **Example**:
  ```bash
  curl -X POST http://localhost:8080/api/subscriptions \
    -H "Content-Type: application/json" \
    -d '{
      "name": "test-1",
      "target_url": "https://jsonplaceholder.typicode.com/posts",
      "payload": {
        "title": "foo",
        "body": "bar",
        "userId": 1
      }
    }'
  ```

#### Get a Subscription

Retrieves details about a specific subscription by ID.

- **URL**: `/api/subscriptions/:id`
- **Method**: `GET`
- **URL Parameters**: `id=[UUID]` - The subscription ID
- **Success Response**:
  - **Code**: `200 OK`
  - **Content**:
    ```json
    {
      "id": "9332827c-7972-454f-bee1-a128047c5557",
      "name": "test-1",
      "target_url": "https://jsonplaceholder.typicode.com/posts",
      "payload": {
        "title": "foo",
        "body": "bar",
        "userId": 1
      },
      "active": true,
      "created_at": "2023-07-01T10:15:30Z",
      "updated_at": "2023-07-01T10:15:30Z"
    }
    ```
- **Example**:
  ```bash
  curl -X GET http://localhost:8080/api/subscriptions/9332827c-7972-454f-bee1-a128047c5557
  ```

#### Update a Subscription

Updates an existing subscription by ID.

- **URL**: `/api/subscriptions/:id`
- **Method**: `PATCH`
- **Content-Type**: `application/json`
- **URL Parameters**: `id=[UUID]` - The subscription ID
- **Request Body**:
  ```json
  {
    "name": "Updated Webhook Name",
    "target_url": "https://jsonplaceholder.typicode.com/posts",
    "payload": {
      "custom_field": "new_value",
      "another_field": 456
    }
  }
  ```
- **Success Response**:
  - **Code**: `200 OK`
  - **Content**:
    ```json
    {
      "id": "9332827c-7972-454f-bee1-a128047c5557",
      "message": "Subscription updated"
    }
    ```
- **Example**:
  ```bash
  curl -X PATCH http://localhost:8080/api/subscriptions/9332827c-7972-454f-bee1-a128047c5557 \
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

Deletes a subscription by ID.

- **URL**: `/api/subscriptions/:id`
- **Method**: `DELETE`
- **URL Parameters**: `id=[UUID]` - The subscription ID
- **Success Response**:
  - **Code**: `200 OK`
  - **Content**:
    ```json
    {
      "message": "Subscription deleted"
    }
    ```
- **Example**:
  ```bash
  curl -X DELETE http://localhost:8080/api/subscriptions/9332827c-7972-454f-bee1-a128047c5557
  ```

### Webhook Delivery

#### Ingest a Webhook

Manually trigger a webhook delivery for a specific subscription.

- **URL**: `/api/ingest/:id`
- **Method**: `POST`
- **Content-Type**: `application/json`
- **URL Parameters**: `id=[UUID]` - The subscription ID
- **Request Body**: Custom payload (optional, overrides subscription payload)
  ```json
  {
    "title": "foo",
    "body": "bar",
    "userId": 1
  }
  ```
- **Success Response**:
  - **Code**: `202 Accepted`
  - **Content**:
    ```json
    {
      "task_id": "4567f21a-9012-45b6-89cd-ef0123456789",
      "message": "Webhook queued for delivery"
    }
    ```
- **Example**:
  ```bash
  curl -X POST http://localhost:8080/api/ingest/9332827c-7972-454f-bee1-a128047c5557 \
    -H "Content-Type: application/json" \
    -d '{
      "title": "foo",
      "body": "bar",
      "userId": 1
    }'
  ```

#### Check Task Status

Get the current status of a delivery task.

- **URL**: `/api/tasks/:id`
- **Method**: `GET`
- **URL Parameters**: `id=[UUID]` - The task ID
- **Success Response**:
  - **Code**: `200 OK`
  - **Content**:
    ```json
    {
      "id": "4567f21a-9012-45b6-89cd-ef0123456789",
      "subscription_id": "9332827c-7972-454f-bee1-a128047c5557",
      "status": "completed",
      "attempt_count": 1,
      "last_attempt": "2023-07-01T10:20:30Z",
      "next_attempt": null
    }
    ```
- **Example**:
  ```bash
  curl -X GET http://localhost:8080/api/tasks/4567f21a-9012-45b6-89cd-ef0123456789
  ```

#### Get Subscription Tasks

Retrieve all tasks related to a subscription.

- **URL**: `/api/subscriptions/:id/tasks`
- **Method**: `GET`
- **URL Parameters**: `id=[UUID]` - The subscription ID
- **Query Parameters**: `limit=[integer]` - Number of tasks to return (default: 20), `offset=[integer]` - Pagination offset (default: 0)
- **Success Response**:
  - **Code**: `200 OK`
  - **Content**:
    ```json
    {
      "tasks": [
        {
          "id": "4567f21a-9012-45b6-89cd-ef0123456789",
          "status": "completed",
          "attempt_count": 1,
          "last_attempt": "2023-07-01T10:20:30Z"
        },
        {
          "id": "7890d45e-3456-78f9-abcd-ef0123456789",
          "status": "failed",
          "attempt_count": 3,
          "last_attempt": "2023-07-01T11:30:45Z"
        }
      ],
      "total": 2,
      "limit": 20,
      "offset": 0
    }
    ```
- **Example**:
  ```bash
  curl -X GET http://localhost:8080/api/subscriptions/9332827c-7972-454f-bee1-a128047c5557/tasks?limit=10
  ```

### Analytics and Logs


#### Get Subscription Logs

Retrieve logs for all tasks related to a subscription.

- **URL**: `/api/subscriptions/:id/logs`
- **Method**: `GET`
- **URL Parameters**: `id=[UUID]` - The subscription ID
- **Query Parameters**: `limit=[integer]` - Number of logs to return (default: 20), `offset=[integer]` - Pagination offset (default: 0)
- **Success Response**:
  - **Code**: `200 OK`
  - **Content**:
    ```json
    {
      "logs": [
        {
          "id": "abc123de-f456-78g9-hijk-lmnopqrst012",
          "task_id": "4567f21a-9012-45b6-89cd-ef0123456789",
          "timestamp": "2023-07-01T10:20:25Z",
          "attempt_number": 1,
          "status": "success",
          "status_code": 201
        },
        {
          "id": "def456gh-i789-01j2-klmn-opqrstu3456",
          "task_id": "7890d45e-3456-78f9-abcd-ef0123456789",
          "timestamp": "2023-07-01T11:30:40Z",
          "attempt_number": 3,
          "status": "failed",
          "status_code": 500
        }
      ],
      "total": 4,
      "limit": 20,
      "offset": 0
    }
    ```
- **Example**:
  ```bash
  curl -X GET http://localhost:8080/api/subscriptions/9332827c-7972-454f-bee1-a128047c5557/logs?limit=10
  ```



#### Get Recent Delivery Attempts

Get analytics on recent delivery attempts for a subscription.

- **URL**: `/api/analytics/subscriptions/:id/recent-attempts`
- **Method**: `GET`
- **URL Parameters**: `id=[UUID]` - The subscription ID
- **Query Parameters**: `limit=[integer]` - Number of attempts to return (default: 10)
- **Success Response**:
  - **Code**: `200 OK`
  - **Content**:
    ```json
    {
      "subscription_id": "9332827c-7972-454f-bee1-a128047c5557",
      "attempts": [
        {
          "task_id": "4567f21a-9012-45b6-89cd-ef0123456789",
          "timestamp": "2023-07-01T10:20:25Z",
          "status": "success",
          "status_code": 201,
          "response_time": 245
        },
        {
          "task_id": "7890d45e-3456-78f9-abcd-ef0123456789",
          "timestamp": "2023-07-01T11:30:40Z",
          "status": "failed",
          "status_code": 500,
          "response_time": 1250
        }
      ],
      "success_rate": 50.0,
      "average_response_time": 747.5
    }
    ```
- **Example**:
  ```bash
  curl -X GET http://localhost:8080/api/analytics/subscriptions/9332827c-7972-454f-bee1-a128047c5557/recent-attempts?limit=10
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

