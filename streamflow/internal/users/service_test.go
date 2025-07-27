package users

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"streamflow/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var testUserService *UserService
var testDbService database.Service

func TestMain(m *testing.M) {
	log.Printf("=== USER SERVICE DATABASE TESTS ===")
	log.Printf("Using real database connection for testing")

	// Set test database name to avoid conflicts with production
	originalDbName := os.Getenv("DB_NAME")
	os.Setenv("DB_NAME", "test_streamflow_users")

	// Check if DB_URI is set
	if os.Getenv("DB_URI") == "" {
		log.Printf("ERROR: DB_URI not set. Please set DB_URI in your .env file")
		log.Printf("Example: DB_URI=mongodb+srv://user:pass@cluster.mongodb.net/dbname")
		os.Exit(1)
	}

	log.Printf("Test database name: test_streamflow_users")

	// Initialize test database service
	testDbService = database.New()
	testUserService = NewUserService(testDbService.GetDatabase())

	code := m.Run()

	// Clean up: Drop the test database to remove all test data
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testDbService.GetDatabase().Drop(ctx)
	testDbService.Close()

	// Restore original database name
	if originalDbName != "" {
		os.Setenv("DB_NAME", originalDbName)
	}

	os.Exit(code)
}

func TestUserService_CreateUser(t *testing.T) {
	t.Log("Testing user creation with real database")

	ctx := context.Background()

	tests := []struct {
		name    string
		req     CreateUserRequest
		wantErr bool
	}{
		{
			name: "valid user creation",
			req: CreateUserRequest{
				UserName: "testuser_" + generateTestSuffix(),
				Email:    "test_" + generateTestSuffix() + "@example.com",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "user with minimum data",
			req: CreateUserRequest{
				UserName: "minuser_" + generateTestSuffix(),
				Email:    "min_" + generateTestSuffix() + "@test.com",
				Password: "pass1234",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := testUserService.CreateUser(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateUser() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("CreateUser() unexpected error = %v", err)
				return
			}

			// Verify user was created correctly
			if user.Email != tt.req.Email {
				t.Errorf("CreateUser() email = %v, want %v", user.Email, tt.req.Email)
			}
			if user.UserName != tt.req.UserName {
				t.Errorf("CreateUser() username = %v, want %v", user.UserName, tt.req.UserName)
			}
			if user.Password == tt.req.Password {
				t.Error("CreateUser() password should be hashed, not plain text")
			}
			if user.ID.IsZero() {
				t.Error("CreateUser() should generate an ID")
			}
			if user.CreatedAt.IsZero() {
				t.Error("CreateUser() should set CreatedAt")
			}

			t.Logf("Successfully created user: %s (%s)", user.UserName, user.Email)
		})
	}
}

func TestUserService_DuplicateUserCreation(t *testing.T) {
	ctx := context.Background()

	// Create a unique test user
	testUser := CreateUserRequest{
		UserName: "duptest_" + generateTestSuffix(),
		Email:    "duptest_" + generateTestSuffix() + "@example.com",
		Password: "password123",
	}

	createdUser, err := testUserService.CreateUser(ctx, testUser)
	if err != nil {
		t.Fatalf("Failed to create initial test user: %v", err)
	}

	t.Logf("Created test user for duplicate testing: %s", createdUser.Email)

	// Try to create user with same email
	duplicateEmailUser := CreateUserRequest{
		UserName: "different_" + generateTestSuffix(),
		Email:    testUser.Email, // Same email
		Password: "password123",
	}

	_, err = testUserService.CreateUser(ctx, duplicateEmailUser)
	if err == nil {
		t.Error("CreateUser() should fail for duplicate email")
	} else {
		t.Logf("Correctly rejected duplicate email: %v", err)
	}

	// Try to create user with same username
	duplicateUsernameUser := CreateUserRequest{
		UserName: testUser.UserName, // Same username
		Email:    "different_" + generateTestSuffix() + "@example.com",
		Password: "password123",
	}

	_, err = testUserService.CreateUser(ctx, duplicateUsernameUser)
	if err == nil {
		t.Error("CreateUser() should fail for duplicate username")
	} else {
		t.Logf("Correctly rejected duplicate username: %v", err)
	}
}

func TestUserService_AuthenticateUser(t *testing.T) {
	ctx := context.Background()

	// First, create a test user
	testUser := CreateUserRequest{
		UserName: "authtest_" + generateTestSuffix(),
		Email:    "auth_" + generateTestSuffix() + "@example.com",
		Password: "password123",
	}
	createdUser, err := testUserService.CreateUser(ctx, testUser)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	t.Logf("Created user for auth testing: %s", createdUser.Email)

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  bool
	}{
		{
			name:     "valid authentication",
			email:    testUser.Email,
			password: "password123",
			wantErr:  false,
		},
		{
			name:     "wrong password",
			email:    testUser.Email,
			password: "wrongpassword",
			wantErr:  true,
		},
		{
			name:     "non-existent email",
			email:    "nonexistent_" + generateTestSuffix() + "@example.com",
			password: "password123",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := testUserService.AuthenticateUser(ctx, tt.email, tt.password)

			if tt.wantErr {
				if err == nil {
					t.Errorf("AuthenticateUser() expected error, got nil")
				} else {
					t.Logf("Correctly rejected invalid auth: %v", err)
				}
				return
			}

			if err != nil {
				t.Errorf("AuthenticateUser() unexpected error = %v", err)
				return
			}

			// Verify returned user matches created user
			if user.ID != createdUser.ID {
				t.Errorf("AuthenticateUser() ID = %v, want %v", user.ID, createdUser.ID)
			}
			if user.Email != createdUser.Email {
				t.Errorf("AuthenticateUser() email = %v, want %v", user.Email, createdUser.Email)
			}

			t.Logf("Successfully authenticated user: %s", user.Email)
		})
	}
}

func TestUserService_GetUserByID(t *testing.T) {
	ctx := context.Background()

	// Create a test user
	testUser := CreateUserRequest{
		UserName: "gettest_" + generateTestSuffix(),
		Email:    "get_" + generateTestSuffix() + "@example.com",
		Password: "password123",
	}
	createdUser, err := testUserService.CreateUser(ctx, testUser)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	t.Logf("Created user for retrieval testing: %s", createdUser.Email)

	// Test valid user ID
	user, err := testUserService.GetUserByID(ctx, createdUser.ID)
	if err != nil {
		t.Errorf("GetUserByID() unexpected error = %v", err)
		return
	}

	if user.ID != createdUser.ID {
		t.Errorf("GetUserByID() ID = %v, want %v", user.ID, createdUser.ID)
	}
	if user.Email != createdUser.Email {
		t.Errorf("GetUserByID() email = %v, want %v", user.Email, createdUser.Email)
	}

	t.Logf("Successfully retrieved user by ID: %s", user.Email)

	// Test non-existent user ID
	_, err = testUserService.GetUserByID(ctx, primitive.NewObjectID())
	if err == nil {
		t.Error("GetUserByID() should fail for non-existent ID")
	} else {
		t.Logf("Correctly handled non-existent user ID: %v", err)
	}
}

func TestUserService_DatabasePersistence(t *testing.T) {
	ctx := context.Background()

	// Create a user
	testUser := CreateUserRequest{
		UserName: "persisttest_" + generateTestSuffix(),
		Email:    "persist_" + generateTestSuffix() + "@example.com",
		Password: "password123",
	}

	createdUser, err := testUserService.CreateUser(ctx, testUser)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	t.Logf("Created user for persistence testing: %s", createdUser.Email)

	// Verify user exists in database by querying directly
	var dbUser User
	err = testUserService.userCollection.FindOne(ctx, bson.M{"_id": createdUser.ID}).Decode(&dbUser)
	if err != nil {
		t.Fatalf("User not found in database: %v", err)
	}

	// Verify all fields are correctly persisted
	if dbUser.Email != testUser.Email {
		t.Errorf("Database email = %v, want %v", dbUser.Email, testUser.Email)
	}
	if dbUser.UserName != testUser.UserName {
		t.Errorf("Database username = %v, want %v", dbUser.UserName, testUser.UserName)
	}
	if dbUser.Password == testUser.Password {
		t.Error("Password should be hashed in database")
	}

	// Test data consistency over time
	time.Sleep(100 * time.Millisecond)

	// Retrieve user again and verify consistency
	retrievedUser, err := testUserService.GetUserByID(ctx, createdUser.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve user after delay: %v", err)
	}

	if retrievedUser.ID != createdUser.ID {
		t.Error("User ID changed after retrieval")
	}
	if retrievedUser.Email != createdUser.Email {
		t.Error("User email changed after retrieval")
	}

	t.Logf("Successfully verified data persistence for user: %s", retrievedUser.Email)
}

// generateTestSuffix creates a unique suffix for test data
func generateTestSuffix() string {
	return time.Now().Format("20060102150405")
}
