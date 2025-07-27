package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Service interface {
	Health() map[string]string
	GetDatabase() *mongo.Database
	Close() error
}

type service struct {
	db *mongo.Client
}

func init() {
	// Try to load .env from current directory first
	if err := godotenv.Load(); err != nil {
		// If not found, try parent directory
		if err := godotenv.Load("../.env"); err != nil {
			// If still not found, try two levels up
			if err := godotenv.Load("../../.env"); err != nil {
				log.Printf("Warning: Could not load .env file from current, parent, or grandparent directory: %v", err)
			}
		}
	}
}

func New() Service {
	uri := os.Getenv("DB_URI")
	if uri == "" {
		// Try to find .env file in common locations
		envPaths := []string{".env", "../.env", "../../.env"}
		for _, path := range envPaths {
			if _, err := os.Stat(path); err == nil {
				log.Printf("Found .env file at: %s", path)
				if err := godotenv.Load(path); err == nil {
					uri = os.Getenv("DB_URI")
					break
				}
			}
		}

		if uri == "" {
			log.Printf("Current working directory: %s", getCurrentDir())
			log.Printf("Checked for .env in: %v", envPaths)
			log.Fatal("You must set your 'DB_URI' environment variable. Make sure .env file is in the correct location.")
		}
	}

	// Use the SetServerAPIOptions() method to set the version of the Stable API on the client
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(uri).SetServerAPIOptions(serverAPI)

	// Create a new client and connect to the server
	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// Send a ping to confirm a successful connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	fmt.Printf("Successfully connected to MongoDB using DB_URI from environment!\n")

	return &service{
		db: client,
	}
}

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
}

func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.db.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Printf("MongoDB health check failed: %v", err)
		return map[string]string{
			"message": "Database is unhealthy",
			"error":   err.Error(),
		}
	}

	return map[string]string{
		"message": "Database is healthy",
		"status":  "connected",
	}
}

func (s *service) GetDatabase() *mongo.Database {
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "streamflow" // default database name
	}
	return s.db.Database(dbName)
}

func (s *service) Close() error {
	if s.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.db.Disconnect(ctx)
	}
	return nil
}
