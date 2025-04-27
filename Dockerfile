FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o segwise-app

# Final lightweight image
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/segwise-app .
COPY --from=builder /app/migrations ./migrations

# Create required directories
RUN mkdir -p /app/logs

# Install necessary tools
RUN apk --no-cache add ca-certificates tzdata

# Set environment variables
ENV TZ=UTC

# Expose the application port
EXPOSE 8080

# Run the application
CMD ["./segwise-app"]
