package database

import (
	"context"
	"log"
	"os"
	"runtime"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestMain(m *testing.M) {
	log.Printf("=== DATABASE INTEGRATION TESTS ===")
	log.Printf("OS: %s", runtime.GOOS)
	log.Printf("Using real database connection for testing")

	// Set test database name to avoid conflicts with production
	originalDbName := os.Getenv("DB_NAME")
	os.Setenv("DB_NAME", "test_streamflow_integration")

	// Check if DB_URI is set
	if os.Getenv("DB_URI") == "" {
		log.Printf("ERROR: DB_URI not set. Please set DB_URI in your .env file")
		log.Printf("Example: DB_URI=mongodb+srv://user:pass@cluster.mongodb.net/dbname")
		os.Exit(1)
	}

	log.Printf("Using real database: %s", maskConnectionString(os.Getenv("DB_URI")))
	log.Printf("Test database name: test_streamflow_integration")

	code := m.Run()

	// Restore original database name
	if originalDbName != "" {
		os.Setenv("DB_NAME", originalDbName)
	}

	os.Exit(code)
}

// maskConnectionString hides the password in connection strings for logging
func maskConnectionString(uri string) string {
	// Simple masking - replace everything between :// and @ with ***
	if len(uri) > 20 {
		return uri[:10] + "***" + uri[len(uri)-20:]
	}
	return "***"
}

func TestNew(t *testing.T) {
	t.Log("Testing with real database connection")

	srv := New()
	if srv == nil {
		t.Fatal("New() returned nil")
	}
}

func TestHealth(t *testing.T) {
	srv := New()

	stats := srv.Health()

	// Test that health check returns expected response
	if stats["message"] != "Database is healthy" {
		t.Errorf("expected message to be 'Database is healthy', got %s", stats["message"])
	}
	if stats["status"] != "connected" {
		t.Errorf("expected status to be 'connected', got %s", stats["status"])
	}
}

func TestDatabaseConnectivity(t *testing.T) {
	srv := New()
	db := srv.GetDatabase()

	if db == nil {
		t.Fatal("GetDatabase() returned nil")
	}

	// Test basic database operations
	ctx := context.Background()

	// Test collection creation and basic operations
	testCollection := db.Collection("test_connectivity")

	// Insert a test document
	testDoc := bson.M{
		"test":      "connectivity",
		"timestamp": time.Now(),
		"os":        runtime.GOOS,
		"test_run":  time.Now().Unix(),
	}
	result, err := testCollection.InsertOne(ctx, testDoc)
	if err != nil {
		t.Errorf("Failed to insert test document: %v", err)
	}

	if result.InsertedID == nil {
		t.Error("InsertedID should not be nil")
	}

	// Query the test document
	var retrievedDoc bson.M
	err = testCollection.FindOne(ctx, bson.M{"test": "connectivity", "test_run": testDoc["test_run"]}).Decode(&retrievedDoc)
	if err != nil {
		t.Errorf("Failed to find test document: %v", err)
	}

	if retrievedDoc["test"] != "connectivity" {
		t.Errorf("Retrieved document test field = %v, want 'connectivity'", retrievedDoc["test"])
	}

	// Clean up test document
	_, err = testCollection.DeleteOne(ctx, bson.M{"test": "connectivity", "test_run": testDoc["test_run"]})
	if err != nil {
		t.Errorf("Failed to delete test document: %v", err)
	}

	t.Logf("Successfully tested database connectivity on %s", runtime.GOOS)
}

func TestDatabaseClose(t *testing.T) {
	srv := New()

	// Test that Close method exists and works
	err := srv.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// After closing, health check should fail
	stats := srv.Health()
	if stats["message"] == "Database is healthy" {
		t.Error("Health check should fail after database is closed")
	}
}

func TestDatabaseReconnection(t *testing.T) {
	// Create a new service instance
	srv1 := New()

	// Verify it works
	stats1 := srv1.Health()
	if stats1["message"] != "Database is healthy" {
		t.Errorf("First connection health = %v, want 'Database is healthy'", stats1["message"])
	}

	// Close first connection
	err := srv1.Close()
	if err != nil {
		t.Errorf("Failed to close first connection: %v", err)
	}

	// Create a new service instance (simulating reconnection)
	srv2 := New()

	// Verify second connection works
	stats2 := srv2.Health()
	if stats2["message"] != "Database is healthy" {
		t.Errorf("Second connection health = %v, want 'Database is healthy'", stats2["message"])
	}

	// Clean up
	srv2.Close()
}
