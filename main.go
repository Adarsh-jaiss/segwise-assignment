package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/adarsh-jaiss/segwise/internal/api"
	"github.com/adarsh-jaiss/segwise/internal/cache"
	"github.com/adarsh-jaiss/segwise/internal/db"
	"github.com/adarsh-jaiss/segwise/internal/worker"

	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var REDDIS_CONN_ADDR = os.Getenv("REDIS_CONN_ADDR")
var REDDIS_PASSWORD = os.Getenv("REDIS_PASSWORD")
var REDDIS_USERNAME = os.Getenv("REDDIS_USERNAME")


func main() {
	
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	dbConn, err := db.ConnectPostgres()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	fmt.Println("Connected to database successfully")

	if err := db.MigrateDB(dbConn); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	fmt.Println("Database migrated successfully")

	redisClient, err := cache.NewRedisClient(REDDIS_CONN_ADDR, REDDIS_PASSWORD, REDDIS_USERNAME)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	fmt.Println("Connected to Redis successfully")
	if redisClient == nil {
		log.Fatalf("Failed to connect to Redis")
	}

	ctx := context.Background()
	worker.InitScheduler(ctx, 5, redisClient, dbConn)
	fmt.Println("Webhook delivery scheduler started with 5 workers")

	fmt.Println("Starting server...")

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.PUT, echo.PATCH, echo.POST, echo.DELETE, echo.OPTIONS},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
		ExposeHeaders: []string{"Content-Length"},
		MaxAge: 86400,
	}))

	api.RegisterRoutes(e, dbConn, redisClient)

	// Add graceful shutdown for the webhook workers
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("Shutting down webhook workers...")
		worker.GlobalScheduler.Stop()
		fmt.Println("Server shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.Shutdown(ctx); err != nil {
			e.Logger.Fatal(err)
		}
	}()

	e.Logger.Fatal(e.Start(":8080"))
}
