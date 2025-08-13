package database

import (
	"context"
	"fmt"
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

// ===== COMPREHENSIVE ADDITIONAL TESTS =====

func TestMultipleConcurrentConnections(t *testing.T) {
	t.Log("Testing multiple concurrent database connections")
	
	const numConnections = 10
	results := make(chan error, numConnections)
	
	// Create multiple concurrent connections
	for i := 0; i < numConnections; i++ {
		go func(id int) {
			t.Logf("Creating connection %d", id)
			srv := New()
			
			// Test health check
			stats := srv.Health()
			if stats["message"] != "Database is healthy" {
				results <- fmt.Errorf("connection %d health check failed: %v", id, stats["message"])
				return
			}
			
			// Test database operations
			db := srv.GetDatabase()
			testCollection := db.Collection("test_concurrent")
			
			testDoc := bson.M{
				"connection_id": id,
				"timestamp":     time.Now(),
				"test_type":     "concurrent",
			}
			
			ctx := context.Background()
			_, err := testCollection.InsertOne(ctx, testDoc)
			if err != nil {
				results <- fmt.Errorf("connection %d insert failed: %v", id, err)
				return
			}
			
			// Clean up
			_, _ = testCollection.DeleteOne(ctx, bson.M{"connection_id": id, "test_type": "concurrent"})
			srv.Close()
			results <- nil
		}(i)
	}
	
	// Wait for all connections to complete
	for i := 0; i < numConnections; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent connection test failed: %v", err)
		}
	}
	
	t.Log("Successfully tested multiple concurrent connections")
}

func TestConnectionTimeout(t *testing.T) {
	t.Log("Testing connection timeout handling")
	
	// Save original environment variables
	originalURI := os.Getenv("DB_URI")
	originalDBName := os.Getenv("DB_NAME")
	
	defer func() {
		// Restore original values
		os.Setenv("DB_URI", originalURI)
		os.Setenv("DB_NAME", originalDBName)
	}()
	
	// Test with invalid connection string (should cause timeout)
	os.Setenv("DB_URI", "mongodb://invalid-host:27017/test")
	os.Setenv("DB_NAME", "test_timeout")
	
	// This should fail during New() call
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected New() to fail with invalid connection string")
		} else {
			t.Logf("Correctly handled invalid connection: %v", r)
		}
	}()
	
	New() // This should panic due to connection failure
}

func TestHealthCheckUnderLoad(t *testing.T) {
	t.Log("Testing health checks under database load")
	
	srv := New()
	defer srv.Close()
	
	db := srv.GetDatabase()
	testCollection := db.Collection("test_load")
	ctx := context.Background()
	
	// Create load by inserting many documents concurrently
	const numOperations = 50
	loadResults := make(chan error, numOperations)
	
	for i := 0; i < numOperations; i++ {
		go func(id int) {
			testDoc := bson.M{
				"load_test_id": id,
				"timestamp":    time.Now(),
				"data":         fmt.Sprintf("load_test_data_%d", id),
			}
			_, err := testCollection.InsertOne(ctx, testDoc)
			loadResults <- err
		}(i)
	}
	
	// Perform health checks while under load
	healthCheckResults := make(chan map[string]string, 10)
	for i := 0; i < 10; i++ {
		go func() {
			time.Sleep(time.Duration(i*10) * time.Millisecond)
			healthCheckResults <- srv.Health()
		}()
	}
	
	// Wait for load operations to complete
	for i := 0; i < numOperations; i++ {
		if err := <-loadResults; err != nil {
			t.Errorf("Load operation %d failed: %v", i, err)
		}
	}
	
	// Check health check results
	for i := 0; i < 10; i++ {
		stats := <-healthCheckResults
		if stats["message"] != "Database is healthy" {
			t.Errorf("Health check failed under load: %v", stats)
		}
	}
	
	// Clean up
	_, _ = testCollection.DeleteMany(ctx, bson.M{"load_test_id": bson.M{"$exists": true}})
	
	t.Log("Successfully tested health checks under load")
}

func TestDatabaseOperationsAfterClose(t *testing.T) {
	t.Log("Testing database operations after connection close")
	
	srv := New()
	
	// Verify initial health
	stats := srv.Health()
	if stats["message"] != "Database is healthy" {
		t.Errorf("Initial health check failed: %v", stats["message"])
	}
	
	// Close the connection
	err := srv.Close()
	if err != nil {
		t.Errorf("Failed to close connection: %v", err)
	}
	
	// Health check should now fail
	stats = srv.Health()
	if stats["message"] == "Database is healthy" {
		t.Error("Health check should fail after close")
	}
	
	// GetDatabase should still return a database object, but operations should fail
	db := srv.GetDatabase()
	if db == nil {
		t.Error("GetDatabase() should not return nil even after close")
	}
	
	t.Log("Successfully tested operations after close")
}

func TestConcurrentReadWriteOperations(t *testing.T) {
	t.Log("Testing concurrent read/write operations")
	
	srv := New()
	defer srv.Close()
	
	db := srv.GetDatabase()
	testCollection := db.Collection("test_concurrent_rw")
	ctx := context.Background()
	
	const numWriters = 10
	const numReaders = 10
	const documentsPerWriter = 5
	
	// Channel to coordinate between writers and readers
	writeComplete := make(chan bool, numWriters)
	readResults := make(chan error, numReaders)
	writeResults := make(chan error, numWriters)
	
	// Start writers
	for w := 0; w < numWriters; w++ {
		go func(writerID int) {
			for d := 0; d < documentsPerWriter; d++ {
				testDoc := bson.M{
					"writer_id":   writerID,
					"document_id": d,
					"timestamp":   time.Now(),
					"data":        fmt.Sprintf("writer_%d_doc_%d", writerID, d),
				}
				
				_, err := testCollection.InsertOne(ctx, testDoc)
				if err != nil {
					writeResults <- fmt.Errorf("writer %d failed on document %d: %v", writerID, d, err)
					return
				}
				
				// Small delay to simulate real workload
				time.Sleep(time.Millisecond * 10)
			}
			writeResults <- nil
			writeComplete <- true
		}(w)
	}
	
	// Start readers
	for r := 0; r < numReaders; r++ {
		go func(readerID int) {
			// Wait a bit for some data to be written
			time.Sleep(time.Millisecond * 50)
			
			for i := 0; i < 10; i++ {
				cursor, err := testCollection.Find(ctx, bson.M{})
				if err != nil {
					readResults <- fmt.Errorf("reader %d failed on iteration %d: %v", readerID, i, err)
					return
				}
				
				var results []bson.M
				if err = cursor.All(ctx, &results); err != nil {
					readResults <- fmt.Errorf("reader %d failed to decode on iteration %d: %v", readerID, i, err)
					cursor.Close(ctx)
					return
				}
				cursor.Close(ctx)
				
				t.Logf("Reader %d found %d documents on iteration %d", readerID, len(results), i)
				time.Sleep(time.Millisecond * 20)
			}
			readResults <- nil
		}(r)
	}
	
	// Wait for all operations to complete
	for w := 0; w < numWriters; w++ {
		if err := <-writeResults; err != nil {
			t.Errorf("Write operation failed: %v", err)
		}
	}
	
	for r := 0; r < numReaders; r++ {
		if err := <-readResults; err != nil {
			t.Errorf("Read operation failed: %v", err)
		}
	}
	
	// Clean up
	_, _ = testCollection.DeleteMany(ctx, bson.M{"writer_id": bson.M{"$exists": true}})
	
	t.Log("Successfully tested concurrent read/write operations")
}

func TestLargeDocumentOperations(t *testing.T) {
	t.Log("Testing large document operations")
	
	srv := New()
	defer srv.Close()
	
	db := srv.GetDatabase()
	testCollection := db.Collection("test_large_docs")
	ctx := context.Background()
	
	// Create a large document (but within MongoDB limits)
	largeData := make([]byte, 1024*1024) // 1MB of data
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	
	largeDoc := bson.M{
		"test_type":  "large_document",
		"timestamp":  time.Now(),
		"large_data": largeData,
		"metadata": bson.M{
			"size":        len(largeData),
			"description": "Large document test",
		},
	}
	
	// Insert large document
	result, err := testCollection.InsertOne(ctx, largeDoc)
	if err != nil {
		t.Errorf("Failed to insert large document: %v", err)
	}
	
	if result.InsertedID == nil {
		t.Error("InsertedID should not be nil for large document")
	}
	
	// Retrieve large document
	var retrievedDoc bson.M
	err = testCollection.FindOne(ctx, bson.M{"test_type": "large_document"}).Decode(&retrievedDoc)
	if err != nil {
		t.Errorf("Failed to retrieve large document: %v", err)
	}
	
	// Verify document integrity
	if retrievedDoc["test_type"] != "large_document" {
		t.Error("Large document test_type mismatch")
	}
	
	// Clean up
	_, _ = testCollection.DeleteOne(ctx, bson.M{"test_type": "large_document"})
	
	t.Log("Successfully tested large document operations")
}

func TestEmptyCollectionOperations(t *testing.T) {
	t.Log("Testing operations on empty collections")
	
	srv := New()
	defer srv.Close()
	
	db := srv.GetDatabase()
	emptyCollection := db.Collection("test_empty_collection")
	ctx := context.Background()
	
	// Ensure collection is empty
	_, _ = emptyCollection.DeleteMany(ctx, bson.M{})
	
	// Test FindOne on empty collection
	var result bson.M
	err := emptyCollection.FindOne(ctx, bson.M{}).Decode(&result)
	if err == nil {
		t.Error("FindOne on empty collection should return an error")
	}
	
	// Test Find on empty collection
	cursor, err := emptyCollection.Find(ctx, bson.M{})
	if err != nil {
		t.Errorf("Find on empty collection should not error: %v", err)
	} else {
		var results []bson.M
		err = cursor.All(ctx, &results)
		if err != nil {
			t.Errorf("Cursor.All() on empty collection failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Expected 0 results from empty collection, got %d", len(results))
		}
		cursor.Close(ctx)
	}
	
	// Test CountDocuments on empty collection
	count, err := emptyCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		t.Errorf("CountDocuments on empty collection failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0 from empty collection, got %d", count)
	}
	
	t.Log("Successfully tested empty collection operations")
}

func TestMultipleServiceInstances(t *testing.T) {
	t.Log("Testing multiple service instances sharing database access")
	
	const numInstances = 5
	services := make([]Service, numInstances)
	
	// Create multiple service instances
	for i := 0; i < numInstances; i++ {
		services[i] = New()
		
		// Verify each instance is healthy
		stats := services[i].Health()
		if stats["message"] != "Database is healthy" {
			t.Errorf("Service instance %d health check failed: %v", i, stats["message"])
		}
	}
	
	// Test concurrent operations from different instances
	ctx := context.Background()
	results := make(chan error, numInstances)
	
	for i, srv := range services {
		go func(instanceID int, service Service) {
			db := service.GetDatabase()
			testCollection := db.Collection("test_multi_instance")
			
			testDoc := bson.M{
				"instance_id": instanceID,
				"timestamp":   time.Now(),
				"test_type":   "multi_instance",
			}
			
			_, err := testCollection.InsertOne(ctx, testDoc)
			if err != nil {
				results <- fmt.Errorf("instance %d insert failed: %v", instanceID, err)
				return
			}
			
			// Verify we can read from the same collection
			var readDoc bson.M
			err = testCollection.FindOne(ctx, bson.M{"instance_id": instanceID}).Decode(&readDoc)
			if err != nil {
				results <- fmt.Errorf("instance %d read failed: %v", instanceID, err)
				return
			}
			
			results <- nil
		}(i, srv)
	}
	
	// Wait for all operations to complete
	for i := 0; i < numInstances; i++ {
		if err := <-results; err != nil {
			t.Errorf("Multi-instance test failed: %v", err)
		}
	}
	
	// Clean up all services
	for i, srv := range services {
		if err := srv.Close(); err != nil {
			t.Errorf("Failed to close service instance %d: %v", i, err)
		}
	}
	
	// Clean up test documents
	cleanupSrv := New()
	db := cleanupSrv.GetDatabase()
	testCollection := db.Collection("test_multi_instance")
	_, _ = testCollection.DeleteMany(ctx, bson.M{"test_type": "multi_instance"})
	cleanupSrv.Close()
	
	t.Log("Successfully tested multiple service instances")
}

func TestDatabaseNameHandling(t *testing.T) {
	t.Log("Testing database name handling with different DB_NAME values")
	
	// Save original DB_NAME
	originalDBName := os.Getenv("DB_NAME")
	defer func() {
		if originalDBName != "" {
			os.Setenv("DB_NAME", originalDBName)
		} else {
			os.Unsetenv("DB_NAME")
		}
	}()
	
	// Test with custom database name
	customDBName := "test_custom_db_name"
	os.Setenv("DB_NAME", customDBName)
	
	srv := New()
	defer srv.Close()
	
	db := srv.GetDatabase()
	if db.Name() != customDBName {
		t.Errorf("Expected database name %s, got %s", customDBName, db.Name())
	}
	
	// Test database operations with custom name
	testCollection := db.Collection("test_custom_name")
	ctx := context.Background()
	
	testDoc := bson.M{
		"test_type": "custom_db_name",
		"db_name":   customDBName,
		"timestamp": time.Now(),
	}
	
	_, err := testCollection.InsertOne(ctx, testDoc)
	if err != nil {
		t.Errorf("Failed to insert document in custom database: %v", err)
	}
	
	// Clean up
	_, _ = testCollection.DeleteOne(ctx, bson.M{"test_type": "custom_db_name"})
	
	t.Log("Successfully tested database name handling")
}

func TestHealthCheckWithContextTimeout(t *testing.T) {
	t.Log("Testing health check behavior with various timeout scenarios")
	
	srv := New()
	defer srv.Close()
	
	// Normal health check should work
	stats := srv.Health()
	if stats["message"] != "Database is healthy" {
		t.Errorf("Initial health check failed: %v", stats["message"])
	}
	
	// Test multiple rapid health checks
	const numHealthChecks = 20
	healthResults := make(chan map[string]string, numHealthChecks)
	
	for i := 0; i < numHealthChecks; i++ {
		go func() {
			healthResults <- srv.Health()
		}()
	}
	
	// Collect results
	for i := 0; i < numHealthChecks; i++ {
		stats := <-healthResults
		if stats["message"] != "Database is healthy" {
			t.Errorf("Rapid health check %d failed: %v", i, stats["message"])
		}
	}
	
	t.Log("Successfully tested health check with various scenarios")
}

func TestConnectionPooling(t *testing.T) {
	t.Log("Testing connection pooling behavior")
	
	srv := New()
	defer srv.Close()
	
	db := srv.GetDatabase()
	ctx := context.Background()
	
	// Simulate multiple operations that would use connection pooling
	const numOperations = 30
	results := make(chan error, numOperations)
	
	for i := 0; i < numOperations; i++ {
		go func(opID int) {
			collection := db.Collection("test_pool")
			
			// Perform multiple operations per goroutine
			for j := 0; j < 3; j++ {
				testDoc := bson.M{
					"operation_id": opID,
					"sub_op":       j,
					"timestamp":    time.Now(),
					"test_type":    "connection_pool",
				}
				
				_, err := collection.InsertOne(ctx, testDoc)
				if err != nil {
					results <- fmt.Errorf("operation %d-%d failed: %v", opID, j, err)
					return
				}
				
				// Small delay to ensure operations overlap
				time.Sleep(time.Millisecond * 5)
			}
			results <- nil
		}(i)
	}
	
	// Wait for all operations
	for i := 0; i < numOperations; i++ {
		if err := <-results; err != nil {
			t.Errorf("Connection pooling test failed: %v", err)
		}
	}
	
	// Verify all documents were inserted
	collection := db.Collection("test_pool")
	count, err := collection.CountDocuments(ctx, bson.M{"test_type": "connection_pool"})
	if err != nil {
		t.Errorf("Failed to count documents: %v", err)
	}
	
	expectedCount := int64(numOperations * 3)
	if count != expectedCount {
		t.Errorf("Expected %d documents, found %d", expectedCount, count)
	}
	
	// Clean up
	_, _ = collection.DeleteMany(ctx, bson.M{"test_type": "connection_pool"})
	
	t.Log("Successfully tested connection pooling")
}

func TestResourceCleanup(t *testing.T) {
	t.Log("Testing proper resource cleanup")
	
	const numIterations = 10
	
	for i := 0; i < numIterations; i++ {
		srv := New()
		
		// Perform some operations
		db := srv.GetDatabase()
		testCollection := db.Collection("test_cleanup")
		ctx := context.Background()
		
		testDoc := bson.M{
			"iteration":  i,
			"timestamp":  time.Now(),
			"test_type":  "resource_cleanup",
		}
		
		_, err := testCollection.InsertOne(ctx, testDoc)
		if err != nil {
			t.Errorf("Failed to insert document in iteration %d: %v", i, err)
		}
		
		// Health check
		stats := srv.Health()
		if stats["message"] != "Database is healthy" {
			t.Errorf("Health check failed in iteration %d: %v", i, stats["message"])
		}
		
		// Clean up resources
		err = srv.Close()
		if err != nil {
			t.Errorf("Failed to close service in iteration %d: %v", i, err)
		}
	}
	
	// Final cleanup of test documents
	cleanupSrv := New()
	defer cleanupSrv.Close()
	
	db := cleanupSrv.GetDatabase()
	testCollection := db.Collection("test_cleanup")
	_, _ = testCollection.DeleteMany(context.Background(), bson.M{"test_type": "resource_cleanup"})
	
	t.Log("Successfully tested resource cleanup")
}

func TestCrossCollectionOperations(t *testing.T) {
	t.Log("Testing operations across multiple collections")
	
	srv := New()
	defer srv.Close()
	
	db := srv.GetDatabase()
	ctx := context.Background()
	
	collections := []string{"test_collection_1", "test_collection_2", "test_collection_3"}
	
	// Insert documents into multiple collections
	for i, collName := range collections {
		collection := db.Collection(collName)
		
		for j := 0; j < 5; j++ {
			testDoc := bson.M{
				"collection_index": i,
				"document_index":   j,
				"collection_name":  collName,
				"timestamp":        time.Now(),
				"test_type":        "cross_collection",
			}
			
			_, err := collection.InsertOne(ctx, testDoc)
			if err != nil {
				t.Errorf("Failed to insert into %s: %v", collName, err)
			}
		}
	}
	
	// Verify documents in each collection
	for _, collName := range collections {
		collection := db.Collection(collName)
		
		count, err := collection.CountDocuments(ctx, bson.M{"test_type": "cross_collection"})
		if err != nil {
			t.Errorf("Failed to count documents in %s: %v", collName, err)
		}
		
		if count != 5 {
			t.Errorf("Expected 5 documents in %s, found %d", collName, count)
		}
		
		// Clean up
		_, _ = collection.DeleteMany(ctx, bson.M{"test_type": "cross_collection"})
	}
	
	t.Log("Successfully tested cross-collection operations")
}

func TestErrorRecovery(t *testing.T) {
	t.Log("Testing error recovery scenarios")
	
	srv := New()
	defer srv.Close()
	
	db := srv.GetDatabase()
	testCollection := db.Collection("test_error_recovery")
	ctx := context.Background()
	
	// Test recovery from invalid operations
	invalidDoc := bson.M{
		"test_type": "error_recovery",
		"timestamp": time.Now(),
	}
	
	// Insert valid document first
	_, err := testCollection.InsertOne(ctx, invalidDoc)
	if err != nil {
		t.Errorf("Failed to insert valid document: %v", err)
	}
	
	// Try to insert document with duplicate _id (should fail)
	invalidDoc["_id"] = "duplicate_id"
	_, err = testCollection.InsertOne(ctx, invalidDoc)
	if err != nil {
		t.Logf("Expected error for duplicate _id: %v", err)
	}
	
	// Verify service is still healthy after error
	stats := srv.Health()
	if stats["message"] != "Database is healthy" {
		t.Errorf("Service should remain healthy after operation error: %v", stats["message"])
	}
	
	// Verify we can still perform valid operations
	validDoc := bson.M{
		"test_type": "error_recovery_valid",
		"timestamp": time.Now(),
	}
	
	_, err = testCollection.InsertOne(ctx, validDoc)
	if err != nil {
		t.Errorf("Failed to insert valid document after error: %v", err)
	}
	
	// Clean up
	_, _ = testCollection.DeleteMany(ctx, bson.M{"test_type": bson.M{"$in": []string{"error_recovery", "error_recovery_valid"}}})
	
	t.Log("Successfully tested error recovery")
}

// Benchmark tests
func BenchmarkHealthCheck(b *testing.B) {
	srv := New()
	defer srv.Close()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		srv.Health()
	}
}

func BenchmarkDatabaseInsert(b *testing.B) {
	srv := New()
	defer srv.Close()
	
	db := srv.GetDatabase()
	testCollection := db.Collection("bench_insert")
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testDoc := bson.M{
			"benchmark": true,
			"iteration": i,
			"timestamp": time.Now(),
		}
		testCollection.InsertOne(ctx, testDoc)
	}
	
	// Clean up
	testCollection.DeleteMany(ctx, bson.M{"benchmark": true})
}

func BenchmarkConcurrentOperations(b *testing.B) {
	srv := New()
	defer srv.Close()
	
	db := srv.GetDatabase()
	testCollection := db.Collection("bench_concurrent")
	ctx := context.Background()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testDoc := bson.M{
				"benchmark":   true,
				"goroutine":   i,
				"timestamp":   time.Now(),
				"concurrent":  true,
			}
			testCollection.InsertOne(ctx, testDoc)
			i++
		}
	})
	
	// Clean up
	testCollection.DeleteMany(ctx, bson.M{"benchmark": true})
}
