#!/bin/bash

# Segwise Webhook Delivery System Test Script
# This script demonstrates the complete workflow of the Segwise system

# Color codes for better readability
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Base URL
BASE_URL="http://localhost:8080"

echo -e "${BLUE}=== Segwise Webhook Delivery System Test ===\n${NC}"

# Step 1: Create a subscription
echo -e "${YELLOW}Creating a new webhook subscription...${NC}"
SUBSCRIPTION_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/subscriptions" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-subscription",
    "target_url": "https://jsonplaceholder.typicode.com/posts",
    "payload": {
      "title": "foo",
      "body": "bar",
      "userId": 1
    }
  }')

# Extract subscription ID from the response
SUBSCRIPTION_ID=$(echo $SUBSCRIPTION_RESPONSE | grep -o '"id":"[^"]*' | cut -d':' -f2 | tr -d '"')

if [ -z "$SUBSCRIPTION_ID" ]; then
  echo -e "${RED}Failed to create subscription. Response: $SUBSCRIPTION_RESPONSE${NC}"
  exit 1
fi

echo -e "${GREEN}Subscription created successfully! ID: $SUBSCRIPTION_ID${NC}\n"

# Step 2: Get subscription details
echo -e "${YELLOW}Retrieving subscription details...${NC}"
SUBSCRIPTION_DETAILS=$(curl -s -X GET "${BASE_URL}/api/subscriptions/$SUBSCRIPTION_ID")
echo -e "${GREEN}Subscription details:${NC}"
echo $SUBSCRIPTION_DETAILS | json_pp
echo ""

# Step 3: Trigger a webhook delivery
echo -e "${YELLOW}Triggering webhook delivery...${NC}"
DELIVERY_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/ingest/$SUBSCRIPTION_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "custom webhook",
    "body": "testing the webhook delivery",
    "userId": 99
  }')

# Extract task ID from the response
TASK_ID=$(echo $DELIVERY_RESPONSE | grep -o '"task_id":"[^"]*' | cut -d':' -f2 | tr -d '"')

if [ -z "$TASK_ID" ]; then
  echo -e "${RED}Failed to trigger webhook. Response: $DELIVERY_RESPONSE${NC}"
  exit 1
fi

echo -e "${GREEN}Webhook triggered successfully! Task ID: $TASK_ID${NC}\n"

# Step 4: Check task status (with retries for processing time)
echo -e "${YELLOW}Checking task status...${NC}"
MAX_RETRIES=10
RETRY_COUNT=0
TASK_STATUS=""

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  TASK_STATUS_RESPONSE=$(curl -s -X GET "${BASE_URL}/api/tasks/$TASK_ID")
  TASK_STATUS=$(echo $TASK_STATUS_RESPONSE | grep -o '"status":"[^"]*' | cut -d':' -f2 | tr -d '"')
  
  if [ "$TASK_STATUS" = "completed" ] || [ "$TASK_STATUS" = "failed" ]; then
    break
  fi
  
  echo -e "Task still processing, waiting 1 second..."
  sleep 1
  RETRY_COUNT=$((RETRY_COUNT + 1))
done

echo -e "${GREEN}Task status:${NC}"
echo $TASK_STATUS_RESPONSE | json_pp
echo ""



# Step 7: Get subscription tasks
echo -e "${YELLOW}Retrieving subscription tasks...${NC}"
SUBSCRIPTION_TASKS=$(curl -s -X GET "${BASE_URL}/api/subscriptions/$SUBSCRIPTION_ID/tasks")
echo -e "${GREEN}Subscription tasks:${NC}"
echo $SUBSCRIPTION_TASKS | json_pp
echo ""

# Step 8: Get recent delivery attempts
echo -e "${YELLOW}Retrieving recent delivery attempts...${NC}"
RECENT_ATTEMPTS=$(curl -s -X GET "${BASE_URL}/api/analytics/subscriptions/$SUBSCRIPTION_ID/recent-attempts?limit=5")
echo -e "${GREEN}Recent delivery attempts:${NC}"
echo $RECENT_ATTEMPTS | json_pp
echo ""

# Optional: Clean up by deleting the subscription
# Uncomment the below lines if you want to delete the test subscription after testing
#echo -e "${YELLOW}Cleaning up: Deleting test subscription...${NC}"
#DELETE_RESPONSE=$(curl -s -X DELETE "${BASE_URL}/api/subscriptions/$SUBSCRIPTION_ID")
#echo -e "${GREEN}Subscription deleted. Response:${NC}"
#echo $DELETE_RESPONSE | json_pp

echo -e "${BLUE}=== Test completed successfully! ===\n${NC}" 