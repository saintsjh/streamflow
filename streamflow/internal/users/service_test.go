package users

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"streamflow/internal/database"

	"github.com/golang-jwt/jwt/v5"
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

// Test JWT Service instance for testing
var testJWTService *JWTService

func init() {
	testJWTService = NewJWTService("test-secret-key-for-testing-purposes")
}

// TestUserService_CreateUser_EdgeCases tests various edge cases for user creation
func TestUserService_CreateUser_EdgeCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		req     CreateUserRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "empty email",
			req: CreateUserRequest{
				UserName: "testuser_" + generateTestSuffix(),
				Email:    "",
				Password: "password123",
			},
			wantErr: true,
			errMsg:  "should fail with empty email",
		},
		{
			name: "invalid email format",
			req: CreateUserRequest{
				UserName: "testuser_" + generateTestSuffix(),
				Email:    "invalid-email",
				Password: "password123",
			},
			wantErr: false, // Service doesn't validate email format, just stores it
		},
		{
			name: "email with special characters",
			req: CreateUserRequest{
				UserName: "testuser_" + generateTestSuffix(),
				Email:    "test+special_" + generateTestSuffix() + "@example.com",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "very long email",
			req: CreateUserRequest{
				UserName: "testuser_" + generateTestSuffix(),
				Email:    "verylongemailaddressthatexceedsnormallimits" + generateTestSuffix() + "@example-domain-with-very-long-name.com",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "empty username",
			req: CreateUserRequest{
				UserName: "",
				Email:    "test_" + generateTestSuffix() + "@example.com",
				Password: "password123",
			},
			wantErr: false, // Service accepts empty username
		},
		{
			name: "unicode username",
			req: CreateUserRequest{
				UserName: "测试用户_" + generateTestSuffix(),
				Email:    "unicode_" + generateTestSuffix() + "@example.com",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "username with special characters",
			req: CreateUserRequest{
				UserName: "test@user#$%_" + generateTestSuffix(),
				Email:    "special_" + generateTestSuffix() + "@example.com",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "very short password",
			req: CreateUserRequest{
				UserName: "testuser_" + generateTestSuffix(),
				Email:    "short_" + generateTestSuffix() + "@example.com",
				Password: "123",
			},
			wantErr: false, // Service doesn't enforce password length
		},
		{
			name: "empty password",
			req: CreateUserRequest{
				UserName: "testuser_" + generateTestSuffix(),
				Email:    "empty_" + generateTestSuffix() + "@example.com",
				Password: "",
			},
			wantErr: false, // bcrypt can hash empty passwords
		},
		{
			name: "very long password",
			req: CreateUserRequest{
				UserName: "testuser_" + generateTestSuffix(),
				Email:    "long_" + generateTestSuffix() + "@example.com",
				Password: "this-is-a-very-long-password-that-exceeds-normal-length-requirements-and-contains-many-characters-to-test-edge-cases-" + generateTestSuffix(),
			},
			wantErr: false,
		},
		{
			name: "password with unicode characters",
			req: CreateUserRequest{
				UserName: "testuser_" + generateTestSuffix(),
				Email:    "unicode_pwd_" + generateTestSuffix() + "@example.com",
				Password: "密码123™€∞",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := testUserService.CreateUser(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateUser() expected error for %s, got nil", tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("CreateUser() unexpected error = %v", err)
				return
			}

			// Verify user creation success
			if user == nil {
				t.Error("CreateUser() returned nil user")
				return
			}

			if user.ID.IsZero() {
				t.Error("CreateUser() should generate an ID")
			}

			t.Logf("Successfully created user with edge case: %s", tt.name)
		})
	}
}

// TestUserService_SQLInjectionPrevention tests SQL injection prevention (though MongoDB uses BSON)
func TestUserService_SQLInjectionPrevention(t *testing.T) {
	ctx := context.Background()

	maliciousInputs := []struct {
		name     string
		email    string
		username string
		password string
	}{
		{
			name:     "email with BSON injection attempt",
			email:    `test@example.com", "$where": "1==1`,
			username: "testuser_" + generateTestSuffix(),
			password: "password123",
		},
		{
			name:     "username with BSON injection",
			email:    "inject_" + generateTestSuffix() + "@example.com",
			username: `{"$ne": null}`,
			password: "password123",
		},
		{
			name:     "password with BSON injection",
			email:    "inject2_" + generateTestSuffix() + "@example.com",
			username: "testuser_" + generateTestSuffix(),
			password: `{"$gt": ""}`,
		},
	}

	for _, tt := range maliciousInputs {
		t.Run(tt.name, func(t *testing.T) {
			req := CreateUserRequest{
				UserName: tt.username,
				Email:    tt.email,
				Password: tt.password,
			}

			user, err := testUserService.CreateUser(ctx, req)
			if err != nil {
				t.Logf("Injection attempt correctly failed: %v", err)
				return
			}

			// If creation succeeded, verify data integrity
			if user.Email != tt.email || user.UserName != tt.username {
				t.Error("User data was modified, possible injection vulnerability")
			}

			// Verify password was hashed properly
			if user.Password == tt.password {
				t.Error("Password should be hashed, possible security issue")
			}

			t.Logf("Injection attempt handled safely: %s", tt.name)
		})
	}
}

// TestUserService_XSSPrevention tests XSS prevention in user data
func TestUserService_XSSPrevention(t *testing.T) {
	ctx := context.Background()

	xssPayloads := []struct {
		name     string
		email    string
		username string
	}{
		{
			name:     "script tag in email",
			email:    "<script>alert('xss')</script>@example.com",
			username: "testuser_" + generateTestSuffix(),
		},
		{
			name:     "script tag in username",
			email:    "xss_" + generateTestSuffix() + "@example.com",
			username: "<script>alert('xss')</script>",
		},
		{
			name:     "javascript in username",
			email:    "js_" + generateTestSuffix() + "@example.com",
			username: "javascript:alert('xss')",
		},
		{
			name:     "html entities",
			email:    "entities_" + generateTestSuffix() + "@example.com",
			username: "&lt;script&gt;alert('xss')&lt;/script&gt;",
		},
	}

	for _, tt := range xssPayloads {
		t.Run(tt.name, func(t *testing.T) {
			req := CreateUserRequest{
				UserName: tt.username,
				Email:    tt.email,
				Password: "password123",
			}

			user, err := testUserService.CreateUser(ctx, req)
			if err != nil {
				t.Logf("XSS payload rejected: %v", err)
				return
			}

			// Verify that data is stored as-is (XSS prevention should happen at output)
			if user.Email != tt.email || user.UserName != tt.username {
				t.Error("User data was unexpectedly modified")
			}

			t.Logf("XSS payload stored safely: %s", tt.name)
		})
	}
}

// TestUserService_AuthenticationSecurity tests authentication security features
func TestUserService_AuthenticationSecurity(t *testing.T) {
	ctx := context.Background()

	// Create test user
	testUser := CreateUserRequest{
		UserName: "authsec_" + generateTestSuffix(),
		Email:    "authsec_" + generateTestSuffix() + "@example.com",
		Password: "password123",
	}
	createdUser, err := testUserService.CreateUser(ctx, testUser)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	t.Run("timing attack resistance", func(t *testing.T) {
		// Test authentication with non-existent user
		start := time.Now()
		_, err1 := testUserService.AuthenticateUser(ctx, "nonexistent@example.com", "password")
		duration1 := time.Since(start)

		// Test authentication with existing user but wrong password
		start = time.Now()
		_, err2 := testUserService.AuthenticateUser(ctx, testUser.Email, "wrongpassword")
		duration2 := time.Since(start)

		// Both should fail
		if err1 == nil || err2 == nil {
			t.Error("Authentication should fail for invalid credentials")
		}

		// Check that both return generic error message
		if err1.Error() != "invalid credentials" || err2.Error() != "invalid credentials" {
			t.Error("Should return generic error message to prevent user enumeration")
		}

		// Timing difference should not be excessive (allowing for some variance)
		timeDiff := duration1 - duration2
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		
		// If timing difference is very large, it might indicate timing attack vulnerability
		if timeDiff > 100*time.Millisecond {
			t.Logf("Warning: Large timing difference detected: %v vs %v", duration1, duration2)
		}
	})

	t.Run("password hashing verification", func(t *testing.T) {
		// Verify password is properly hashed
		if createdUser.Password == testUser.Password {
			t.Error("Password should be hashed, not stored in plaintext")
		}

		// Verify hash starts with bcrypt identifier
		if !strings.HasPrefix(createdUser.Password, "$2") {
			t.Error("Password should be hashed with bcrypt")
		}

		// Verify hash length is appropriate for bcrypt
		if len(createdUser.Password) < 50 {
			t.Error("Bcrypt hash should be at least 50 characters")
		}
	})

	t.Run("case sensitivity", func(t *testing.T) {
		// Test case sensitivity in email (emails should typically be case-insensitive)
		upperEmail := strings.ToUpper(testUser.Email)
		_, err := testUserService.AuthenticateUser(ctx, upperEmail, testUser.Password)
		if err == nil {
			t.Log("Email authentication is case-insensitive")
		} else {
			t.Log("Email authentication is case-sensitive")
		}
	})
}

// TestUserService_PasswordComplexity tests password handling
func TestUserService_PasswordComplexity(t *testing.T) {
	ctx := context.Background()

	passwordTests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "common password",
			password: "password",
			wantErr:  false, // Service doesn't enforce complexity
		},
		{
			name:     "sequential numbers",
			password: "123456789",
			wantErr:  false,
		},
		{
			name:     "keyboard pattern",
			password: "qwertyuiop",
			wantErr:  false,
		},
		{
			name:     "all same character",
			password: "aaaaaaaaaa",
			wantErr:  false,
		},
		{
			name:     "strong password",
			password: "Str0ng!P@ssw0rd#2024",
			wantErr:  false,
		},
	}

	for _, tt := range passwordTests {
		t.Run(tt.name, func(t *testing.T) {
			req := CreateUserRequest{
				UserName: "pwdtest_" + generateTestSuffix(),
				Email:    "pwd_" + generateTestSuffix() + "@example.com",
				Password: tt.password,
			}

			user, err := testUserService.CreateUser(ctx, req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateUser() should reject weak password: %s", tt.password)
				}
				return
			}

			if err != nil {
				t.Errorf("CreateUser() unexpected error = %v", err)
				return
			}

			// Verify password was hashed
			if user.Password == tt.password {
				t.Error("Password should be hashed")
			}

			// Test authentication with the password
			authUser, authErr := testUserService.AuthenticateUser(ctx, req.Email, tt.password)
			if authErr != nil {
				t.Errorf("Authentication failed for valid password: %v", authErr)
			} else if authUser.ID != user.ID {
				t.Error("Authenticated user doesn't match created user")
			}
		})
	}
}

// TestUserService_ConcurrentUserCreation tests concurrent user creation
func TestUserService_ConcurrentUserCreation(t *testing.T) {
	ctx := context.Background()
	
	const numGoroutines = 10
	const usersPerGoroutine = 5

	results := make(chan struct {
		user *User
		err  error
	}, numGoroutines*usersPerGoroutine)

	// Start concurrent user creation
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < usersPerGoroutine; j++ {
				req := CreateUserRequest{
					UserName: fmt.Sprintf("concurrent_%d_%d_%s", goroutineID, j, generateTestSuffix()),
					Email:    fmt.Sprintf("concurrent_%d_%d_%s@example.com", goroutineID, j, generateTestSuffix()),
					Password: "password123",
				}

				user, err := testUserService.CreateUser(ctx, req)
				results <- struct {
					user *User
					err  error
				}{user: user, err: err}
			}
		}(i)
	}

	// Collect results
	successCount := 0
	errorCount := 0
	userIDs := make(map[primitive.ObjectID]bool)

	for i := 0; i < numGoroutines*usersPerGoroutine; i++ {
		result := <-results
		if result.err != nil {
			errorCount++
			t.Logf("Concurrent creation error: %v", result.err)
		} else {
			successCount++
			// Check for duplicate IDs
			if userIDs[result.user.ID] {
				t.Errorf("Duplicate user ID detected: %s", result.user.ID.Hex())
			}
			userIDs[result.user.ID] = true
		}
	}

	t.Logf("Concurrent creation results: %d successful, %d errors", successCount, errorCount)

	if successCount == 0 {
		t.Error("No users were created successfully in concurrent test")
	}
}

// TestUserService_DuplicateHandlingRaceCondition tests race conditions in duplicate detection
func TestUserService_DuplicateHandlingRaceCondition(t *testing.T) {
	ctx := context.Background()
	
	const numGoroutines = 5
	baseEmail := "racetest_" + generateTestSuffix() + "@example.com"
	baseUsername := "racetest_" + generateTestSuffix()

	results := make(chan error, numGoroutines)

	// Try to create the same user concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			req := CreateUserRequest{
				UserName: baseUsername,
				Email:    baseEmail,
				Password: "password123",
			}

			_, err := testUserService.CreateUser(ctx, req)
			results <- err
		}()
	}

	// Collect results
	successCount := 0
	errorCount := 0

	for i := 0; i < numGoroutines; i++ {
		err := <-results
		if err != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	t.Logf("Race condition results: %d successful, %d errors", successCount, errorCount)

	// Only one should succeed, rest should fail due to duplicate detection
	if successCount != 1 {
		t.Errorf("Expected exactly 1 success, got %d", successCount)
	}
	if errorCount != numGoroutines-1 {
		t.Errorf("Expected %d errors, got %d", numGoroutines-1, errorCount)
	}
}

// TestUserService_DatabaseConsistency tests database consistency and integrity
func TestUserService_DatabaseConsistency(t *testing.T) {
	ctx := context.Background()

	t.Run("user data integrity", func(t *testing.T) {
		// Create user with specific data
		req := CreateUserRequest{
			UserName: "integrity_" + generateTestSuffix(),
			Email:    "integrity_" + generateTestSuffix() + "@example.com",
			Password: "password123",
		}

		user, err := testUserService.CreateUser(ctx, req)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		// Retrieve user multiple times and verify consistency
		for i := 0; i < 5; i++ {
			retrievedUser, err := testUserService.GetUserByID(ctx, user.ID)
			if err != nil {
				t.Fatalf("Failed to retrieve user on attempt %d: %v", i+1, err)
			}

			// Verify all fields match
			if retrievedUser.ID != user.ID {
				t.Errorf("ID mismatch on attempt %d", i+1)
			}
			if retrievedUser.Email != user.Email {
				t.Errorf("Email mismatch on attempt %d", i+1)
			}
			if retrievedUser.UserName != user.UserName {
				t.Errorf("Username mismatch on attempt %d", i+1)
			}
			if retrievedUser.Password != user.Password {
				t.Errorf("Password hash mismatch on attempt %d", i+1)
			}
		}
	})

	t.Run("timestamp consistency", func(t *testing.T) {
		beforeCreate := time.Now()
		
		req := CreateUserRequest{
			UserName: "timestamp_" + generateTestSuffix(),
			Email:    "timestamp_" + generateTestSuffix() + "@example.com",
			Password: "password123",
		}

		user, err := testUserService.CreateUser(ctx, req)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		afterCreate := time.Now()

		// Verify timestamps are within reasonable range
		if user.CreatedAt.Before(beforeCreate) || user.CreatedAt.After(afterCreate) {
			t.Error("CreatedAt timestamp is not within expected range")
		}
		if user.UpdatedAt.Before(beforeCreate) || user.UpdatedAt.After(afterCreate) {
			t.Error("UpdatedAt timestamp is not within expected range")
		}

		// CreatedAt and UpdatedAt should be very close for new users
		timeDiff := user.UpdatedAt.Sub(user.CreatedAt)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		if timeDiff > time.Second {
			t.Error("CreatedAt and UpdatedAt should be very close for new users")
		}
	})
}

// TestUserService_GetUserByID_EdgeCases tests edge cases for user retrieval
func TestUserService_GetUserByID_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid object ID", func(t *testing.T) {
		// Test with zero ObjectID
		zeroID := primitive.ObjectID{}
		_, err := testUserService.GetUserByID(ctx, zeroID)
		if err == nil {
			t.Error("GetUserByID should fail for zero ObjectID")
		}
	})

	t.Run("non-existent user ID", func(t *testing.T) {
		// Generate a valid but non-existent ObjectID
		nonExistentID := primitive.NewObjectID()
		_, err := testUserService.GetUserByID(ctx, nonExistentID)
		if err == nil {
			t.Error("GetUserByID should fail for non-existent user")
		}
	})

	t.Run("valid user retrieval", func(t *testing.T) {
		// Create a user first
		req := CreateUserRequest{
			UserName: "gettest_" + generateTestSuffix(),
			Email:    "gettest_" + generateTestSuffix() + "@example.com",
			Password: "password123",
		}

		createdUser, err := testUserService.CreateUser(ctx, req)
		if err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		// Retrieve the user
		retrievedUser, err := testUserService.GetUserByID(ctx, createdUser.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve user: %v", err)
		}

		// Verify all fields match
		if retrievedUser.ID != createdUser.ID {
			t.Error("Retrieved user ID doesn't match")
		}
		if retrievedUser.Email != createdUser.Email {
			t.Error("Retrieved user email doesn't match")
		}
		if retrievedUser.UserName != createdUser.UserName {
			t.Error("Retrieved user username doesn't match")
		}
	})
}

// TestJWTService_TokenGeneration tests JWT token generation
func TestJWTService_TokenGeneration(t *testing.T) {
	ctx := context.Background()

	// Create a test user
	req := CreateUserRequest{
		UserName: "jwttest_" + generateTestSuffix(),
		Email:    "jwt_" + generateTestSuffix() + "@example.com",
		Password: "password123",
	}

	user, err := testUserService.CreateUser(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	t.Run("successful token generation", func(t *testing.T) {
		token, err := testJWTService.GenerateToken(user.ID)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		if token == "" {
			t.Error("Generated token should not be empty")
		}

		// Verify token has three parts (header.payload.signature)
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			t.Error("JWT token should have three parts separated by dots")
		}

		t.Logf("Successfully generated token: %s", token[:20]+"...")
	})

	t.Run("token validation", func(t *testing.T) {
		token, err := testJWTService.GenerateToken(user.ID)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		claims, err := testJWTService.verifyToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}

		if claims.UserID != user.ID.Hex() {
			t.Error("Token claims UserID doesn't match")
		}
		// if claims.Email != user.Email {
		// 	t.Error("Token claims Email doesn't match")
		// }
		if claims.Issuer != "streamflow" {
			t.Error("Token issuer should be 'streamflow'")
		}
	})

	t.Run("token expiration", func(t *testing.T) {
		token, err := testJWTService.GenerateToken(user.ID)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		claims, err := testJWTService.verifyToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}

		// Verify expiration is set to 24 hours from now
		expectedExpiration := time.Now().Add(24 * time.Hour)
		actualExpiration := claims.ExpiresAt.Time

		timeDiff := actualExpiration.Sub(expectedExpiration)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}

		// Allow for small timing differences (up to 1 minute)
		if timeDiff > time.Minute {
			t.Errorf("Token expiration time is not as expected. Diff: %v", timeDiff)
		}
	})
}

// TestJWTService_TokenValidation tests JWT token validation edge cases
func TestJWTService_TokenValidation(t *testing.T) {
	ctx := context.Background()

	// Create a test user
	req := CreateUserRequest{
		UserName: "jwtval_" + generateTestSuffix(),
		Email:    "jwtval_" + generateTestSuffix() + "@example.com",
		Password: "password123",
	}

	user, err := testUserService.CreateUser(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	validToken, err := testJWTService.GenerateToken(user.ID)
	if err != nil {
		t.Fatalf("Failed to generate valid token: %v", err)
	}

	tests := []struct {
		name        string
		token       string
		wantErr     bool
		description string
	}{
		{
			name:        "valid token",
			token:       validToken,
			wantErr:     false,
			description: "should validate successfully",
		},
		{
			name:        "empty token",
			token:       "",
			wantErr:     true,
			description: "should fail for empty token",
		},
		{
			name:        "malformed token - only one part",
			token:       "invalid",
			wantErr:     true,
			description: "should fail for malformed token",
		},
		{
			name:        "malformed token - only two parts",
			token:       "header.payload",
			wantErr:     true,
			description: "should fail for incomplete token",
		},
		{
			name:        "invalid signature",
			token:       validToken[:len(validToken)-5] + "XXXXX",
			wantErr:     true,
			description: "should fail for tampered signature",
		},
		{
			name:        "completely invalid token",
			token:       "not.a.jwt.token.at.all",
			wantErr:     true,
			description: "should fail for completely invalid token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := testJWTService.verifyToken(tt.token)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateToken() %s, got nil", tt.description)
				} else {
					t.Logf("Correctly rejected invalid token: %v", err)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateToken() unexpected error = %v", err)
				return
			}

			if claims == nil {
				t.Error("ValidateToken() returned nil claims for valid token")
			}
		})
	}
}

// TestJWTService_ExpiredToken tests expired token handling
func TestJWTService_ExpiredToken(t *testing.T) {
	// Create a JWT service with very short expiration for testing
	shortExpiryJWT := &JWTService{
		secretKey: "test-secret-key-for-testing-purposes",
	}

	ctx := context.Background()

	// Create a test user
	req := CreateUserRequest{
		UserName: "expiry_" + generateTestSuffix(),
		Email:    "expiry_" + generateTestSuffix() + "@example.com",
		Password: "password123",
	}

	user, err := testUserService.CreateUser(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create a token that's already expired
	expiredClaims := &JWTClaims{
		UserID: user.ID.Hex(),
		// Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // Expired 1 hour ago
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "streamflow",
			Subject:   user.ID.Hex(),
		},
	}

	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredTokenString, err := expiredToken.SignedString(shortExpiryJWT.secretKey)
	if err != nil {
		t.Fatalf("Failed to create expired token: %v", err)
	}

	// Try to validate the expired token
	_, err = shortExpiryJWT.verifyToken(expiredTokenString)
	if err == nil {
		t.Error("ValidateToken should fail for expired token")
	} else {
		t.Logf("Correctly rejected expired token: %v", err)
	}
}

// TestUserService_AuthenticationBruteForce tests brute force protection simulation
func TestUserService_AuthenticationBruteForce(t *testing.T) {
	ctx := context.Background()

	// Create a test user
	req := CreateUserRequest{
		UserName: "brutetest_" + generateTestSuffix(),
		Email:    "brute_" + generateTestSuffix() + "@example.com",
		Password: "password123",
	}

	user, err := testUserService.CreateUser(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Simulate multiple failed login attempts
	wrongPasswords := []string{
		"wrongpass1", "wrongpass2", "wrongpass3", "wrongpass4", "wrongpass5",
		"123456", "password", "admin", "letmein", "qwerty",
	}

	var authTimes []time.Duration

	for i, wrongPass := range wrongPasswords {
		start := time.Now()
		_, err := testUserService.AuthenticateUser(ctx, user.Email, wrongPass)
		duration := time.Since(start)
		authTimes = append(authTimes, duration)

		if err == nil {
			t.Errorf("Authentication should fail for wrong password on attempt %d", i+1)
		}

		// All errors should be generic
		if err.Error() != "invalid credentials" {
			t.Errorf("Should return generic error message on attempt %d", i+1)
		}
	}

	// Check for consistent timing (basic timing attack protection)
	if len(authTimes) > 1 {
		var totalTime time.Duration
		for _, t := range authTimes {
			totalTime += t
		}
		avgTime := totalTime / time.Duration(len(authTimes))

		// Check if any individual time deviates significantly from average
		for i, authTime := range authTimes {
			deviation := authTime - avgTime
			if deviation < 0 {
				deviation = -deviation
			}

			// If deviation is more than 10x the average, it might indicate timing issues
			if deviation > avgTime*10 {
				t.Logf("Warning: Large timing deviation on attempt %d: %v (avg: %v)", i+1, authTime, avgTime)
			}
		}
	}

	// Verify that correct password still works
	_, err = testUserService.AuthenticateUser(ctx, user.Email, "password123")
	if err != nil {
		t.Error("Valid authentication should still work after failed attempts")
	}
}

// TestUserService_InputSanitization tests input sanitization
func TestUserService_InputSanitization(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		input    CreateUserRequest
		expected map[string]string // expected values after sanitization
	}{
		{
			name: "leading/trailing whitespace",
			input: CreateUserRequest{
				UserName: "  testuser  ",
				Email:    "  test@example.com  ",
				Password: "  password123  ",
			},
			expected: map[string]string{
				"username": "  testuser  ", // Service doesn't trim whitespace
				"email":    "  test@example.com  ",
			},
		},
		{
			name: "mixed case email",
			input: CreateUserRequest{
				UserName: "testuser_" + generateTestSuffix(),
				Email:    "TeSt_" + generateTestSuffix() + "@ExAmPlE.CoM",
				Password: "password123",
			},
			expected: map[string]string{
				"email": "TeSt_" + generateTestSuffix() + "@ExAmPlE.CoM", // Service preserves case
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := testUserService.CreateUser(ctx, tt.input)
			if err != nil {
				t.Fatalf("Failed to create user: %v", err)
			}

			// Check that values are stored as expected
			if expectedUsername, ok := tt.expected["username"]; ok {
				if user.UserName != expectedUsername {
					t.Errorf("Username = %q, expected %q", user.UserName, expectedUsername)
				}
			}

			if expectedEmail, ok := tt.expected["email"]; ok {
				if user.Email != expectedEmail {
					t.Errorf("Email = %q, expected %q", user.Email, expectedEmail)
				}
			}

			// Password should always be hashed
			if user.Password == tt.input.Password {
				t.Error("Password should be hashed, not stored as plaintext")
			}
		})
	}
}

// TestUserService_DatabaseErrorHandling tests database error scenarios
func TestUserService_DatabaseErrorHandling(t *testing.T) {
	ctx := context.Background()

	t.Run("context cancellation", func(t *testing.T) {
		// Create a context that's already cancelled
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		req := CreateUserRequest{
			UserName: "cancelled_" + generateTestSuffix(),
			Email:    "cancelled_" + generateTestSuffix() + "@example.com",
			Password: "password123",
		}

		_, err := testUserService.CreateUser(cancelledCtx, req)
		if err == nil {
			t.Error("CreateUser should fail with cancelled context")
		} else {
			t.Logf("Correctly handled cancelled context: %v", err)
		}
	})

	t.Run("context timeout", func(t *testing.T) {
		// Create a context with very short timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
		defer cancel()

		// Wait a bit to ensure timeout
		time.Sleep(10 * time.Millisecond)

		req := CreateUserRequest{
			UserName: "timeout_" + generateTestSuffix(),
			Email:    "timeout_" + generateTestSuffix() + "@example.com",
			Password: "password123",
		}

		_, err := testUserService.CreateUser(timeoutCtx, req)
		if err == nil {
			t.Error("CreateUser should fail with timeout context")
		} else {
			t.Logf("Correctly handled timeout context: %v", err)
		}
	})
}

// TestUserService_PerformanceBasic tests basic performance characteristics
func TestUserService_PerformanceBasic(t *testing.T) {
	ctx := context.Background()

	t.Run("user creation performance", func(t *testing.T) {
		const numUsers = 50
		
		start := time.Now()
		
		for i := 0; i < numUsers; i++ {
			req := CreateUserRequest{
				UserName: fmt.Sprintf("perf_%d_%s", i, generateTestSuffix()),
				Email:    fmt.Sprintf("perf_%d_%s@example.com", i, generateTestSuffix()),
				Password: "password123",
			}

			_, err := testUserService.CreateUser(ctx, req)
			if err != nil {
				t.Errorf("Failed to create user %d: %v", i, err)
			}
		}

		duration := time.Since(start)
		avgTimePerUser := duration / numUsers

		t.Logf("Created %d users in %v (avg: %v per user)", numUsers, duration, avgTimePerUser)

		// Basic performance check - should be reasonable for a database operation
		if avgTimePerUser > 5*time.Second {
			t.Errorf("User creation taking too long: %v per user", avgTimePerUser)
		}
	})

	t.Run("authentication performance", func(t *testing.T) {
		// Create a test user first
		req := CreateUserRequest{
			UserName: "authperf_" + generateTestSuffix(),
			Email:    "authperf_" + generateTestSuffix() + "@example.com",
			Password: "password123",
		}

		user, err := testUserService.CreateUser(ctx, req)
		if err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		const numAuths = 20
		
		start := time.Now()
		
		for i := 0; i < numAuths; i++ {
			_, err := testUserService.AuthenticateUser(ctx, user.Email, "password123")
			if err != nil {
				t.Errorf("Authentication failed on attempt %d: %v", i, err)
			}
		}

		duration := time.Since(start)
		avgTimePerAuth := duration / numAuths

		t.Logf("Performed %d authentications in %v (avg: %v per auth)", numAuths, duration, avgTimePerAuth)

		// Authentication should be reasonably fast
		if avgTimePerAuth > 2*time.Second {
			t.Errorf("Authentication taking too long: %v per auth", avgTimePerAuth)
		}
	})
}

// TestUserService_EmailValidation tests email validation edge cases
func TestUserService_EmailValidation(t *testing.T) {
	ctx := context.Background()

	emailTests := []struct {
		name    string
		email   string
		wantErr bool
		desc    string
	}{
		{
			name:    "valid email",
			email:   "valid_" + generateTestSuffix() + "@example.com",
			wantErr: false,
			desc:    "should accept valid email",
		},
		{
			name:    "email with plus sign",
			email:   "test+tag_" + generateTestSuffix() + "@example.com",
			wantErr: false,
			desc:    "should accept email with plus sign",
		},
		{
			name:    "email with dots",
			email:   "test.user_" + generateTestSuffix() + "@example.com",
			wantErr: false,
			desc:    "should accept email with dots",
		},
		{
			name:    "email with subdomain",
			email:   "test_" + generateTestSuffix() + "@mail.example.com",
			wantErr: false,
			desc:    "should accept email with subdomain",
		},
		{
			name:    "email with numbers",
			email:   "test123_" + generateTestSuffix() + "@example123.com",
			wantErr: false,
			desc:    "should accept email with numbers",
		},
		{
			name:    "email with hyphens",
			email:   "test-user_" + generateTestSuffix() + "@ex-ample.com",
			wantErr: false,
			desc:    "should accept email with hyphens",
		},
		{
			name:    "international domain",
			email:   "test_" + generateTestSuffix() + "@example.co.uk",
			wantErr: false,
			desc:    "should accept international domain",
		},
	}

	for _, tt := range emailTests {
		t.Run(tt.name, func(t *testing.T) {
			req := CreateUserRequest{
				UserName: "emailtest_" + generateTestSuffix(),
				Email:    tt.email,
				Password: "password123",
			}

			user, err := testUserService.CreateUser(ctx, req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateUser() %s, got nil", tt.desc)
				}
				return
			}

			if err != nil {
				t.Errorf("CreateUser() unexpected error = %v", err)
				return
			}

			if user.Email != tt.email {
				t.Errorf("Email stored incorrectly: got %s, want %s", user.Email, tt.email)
			}
		})
	}
}

// TestUserService_UsernameConstraints tests username validation
func TestUserService_UsernameConstraints(t *testing.T) {
	ctx := context.Background()

	usernameTests := []struct {
		name     string
		username string
		wantErr  bool
		desc     string
	}{
		{
			name:     "normal username",
			username: "normaluser_" + generateTestSuffix(),
			wantErr:  false,
			desc:     "should accept normal username",
		},
		{
			name:     "username with numbers",
			username: "user123_" + generateTestSuffix(),
			wantErr:  false,
			desc:     "should accept username with numbers",
		},
		{
			name:     "username with underscores",
			username: "user_name_" + generateTestSuffix(),
			wantErr:  false,
			desc:     "should accept username with underscores",
		},
		{
			name:     "username with hyphens",
			username: "user-name-" + generateTestSuffix(),
			wantErr:  false,
			desc:     "should accept username with hyphens",
		},
		{
			name:     "single character username",
			username: "a",
			wantErr:  false,
			desc:     "should handle single character username",
		},
		{
			name:     "very long username",
			username: "verylongusernamethatexceedsnormallengthconstraints" + generateTestSuffix(),
			wantErr:  false,
			desc:     "should handle very long username",
		},
		{
			name:     "username with spaces",
			username: "user name " + generateTestSuffix(),
			wantErr:  false,
			desc:     "should handle username with spaces",
		},
	}

	for _, tt := range usernameTests {
		t.Run(tt.name, func(t *testing.T) {
			req := CreateUserRequest{
				UserName: tt.username,
				Email:    "username_test_" + generateTestSuffix() + "@example.com",
				Password: "password123",
			}

			user, err := testUserService.CreateUser(ctx, req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateUser() %s, got nil", tt.desc)
				}
				return
			}

			if err != nil {
				t.Errorf("CreateUser() unexpected error = %v", err)
				return
			}

			if user.UserName != tt.username {
				t.Errorf("Username stored incorrectly: got %s, want %s", user.UserName, tt.username)
			}
		})
	}
}

// TestJWTService_VerifyToken tests the VerifyToken method specifically
func TestJWTService_VerifyToken(t *testing.T) {
	ctx := context.Background()

	// Create a test user
	req := CreateUserRequest{
		UserName: "verifytest_" + generateTestSuffix(),
		Email:    "verify_" + generateTestSuffix() + "@example.com",
		Password: "password123",
	}

	user, err := testUserService.CreateUser(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	t.Run("verify valid token", func(t *testing.T) {
		token, err := testJWTService.GenerateToken(user.ID)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		claims, err := testJWTService.verifyToken(token)
		if err != nil {
			t.Errorf("VerifyToken() failed for valid token: %v", err)
			return
		}

		if claims.UserID != user.ID.Hex() {
			t.Error("VerifyToken() claims UserID doesn't match")
		}
		// if claims.Email != user.Email {
		// 	t.Error("VerifyToken() claims Email doesn't match")
		// }
	})

	t.Run("verify token with different secret", func(t *testing.T) {
		// Create JWT service with different secret
		differentSecretJWT := NewJWTService("different-secret-key")
		
		token, err := testJWTService.GenerateToken(user.ID)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Try to verify with different secret
		_, err = differentSecretJWT.verifyToken(token)
		if err == nil {
			t.Error("VerifyToken() should fail with different secret")
		} else {
			t.Logf("Correctly rejected token with different secret: %v", err)
		}
	})
}

// TestUserService_StressTest performs a stress test with many operations
func TestUserService_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	ctx := context.Background()
	const numOperations = 100

	t.Run("stress create users", func(t *testing.T) {
		errors := 0
		successes := 0

		for i := 0; i < numOperations; i++ {
			req := CreateUserRequest{
				UserName: fmt.Sprintf("stress_%d_%s", i, generateTestSuffix()),
				Email:    fmt.Sprintf("stress_%d_%s@example.com", i, generateTestSuffix()),
				Password: "password123",
			}

			_, err := testUserService.CreateUser(ctx, req)
			if err != nil {
				errors++
				t.Logf("Error creating user %d: %v", i, err)
			} else {
				successes++
			}
		}

		t.Logf("Stress test results: %d successes, %d errors", successes, errors)

		if successes == 0 {
			t.Error("No users created successfully in stress test")
		}

		// Allow for some errors in stress testing
		if float64(errors)/float64(numOperations) > 0.1 {
			t.Errorf("Too many errors in stress test: %d/%d", errors, numOperations)
		}
	})
}

// TestUserService_AuthenticationStressTest performs authentication stress testing
func TestUserService_AuthenticationStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping authentication stress test in short mode")
	}

	ctx := context.Background()

	// Create test users
	const numUsers = 20
	users := make([]*User, numUsers)
	passwords := make([]string, numUsers)

	for i := 0; i < numUsers; i++ {
		password := fmt.Sprintf("password%d", i)
		req := CreateUserRequest{
			UserName: fmt.Sprintf("authstress_%d_%s", i, generateTestSuffix()),
			Email:    fmt.Sprintf("authstress_%d_%s@example.com", i, generateTestSuffix()),
			Password: password,
		}

		user, err := testUserService.CreateUser(ctx, req)
		if err != nil {
			t.Fatalf("Failed to create test user %d: %v", i, err)
		}

		users[i] = user
		passwords[i] = password
	}

	// Perform many authentication attempts
	const numAuths = 100
	errors := 0
	successes := 0

	for i := 0; i < numAuths; i++ {
		userIndex := i % numUsers
		_, err := testUserService.AuthenticateUser(ctx, users[userIndex].Email, passwords[userIndex])
		if err != nil {
			errors++
			t.Logf("Auth error %d: %v", i, err)
		} else {
			successes++
		}
	}

	t.Logf("Authentication stress test: %d successes, %d errors", successes, errors)

	if successes == 0 {
		t.Error("No authentications succeeded in stress test")
	}

	if float64(errors)/float64(numAuths) > 0.05 {
		t.Errorf("Too many authentication errors: %d/%d", errors, numAuths)
	}
}

// TestUserService_TokenStressTest performs JWT token stress testing
func TestUserService_TokenStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping token stress test in short mode")
	}

	ctx := context.Background()

	// Create a test user
	req := CreateUserRequest{
		UserName: "tokenstress_" + generateTestSuffix(),
		Email:    "tokenstress_" + generateTestSuffix() + "@example.com",
		Password: "password123",
	}

	user, err := testUserService.CreateUser(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	const numTokenOps = 50

	t.Run("generate many tokens", func(t *testing.T) {
		tokens := make([]string, numTokenOps)
		errors := 0

		for i := 0; i < numTokenOps; i++ {
			token, err := testJWTService.GenerateToken(user.ID	)
			if err != nil {
				errors++
				t.Logf("Token generation error %d: %v", i, err)
			} else {
				tokens[i] = token
			}
		}

		if errors > 0 {
			t.Errorf("Token generation errors: %d/%d", errors, numTokenOps)
		}

		// Validate all generated tokens
		validationErrors := 0
		for i, token := range tokens {
			if token == "" {
				continue
			}

			claims, err := testJWTService.verifyToken(token)
			if err != nil {
				validationErrors++
				t.Logf("Token validation error %d: %v", i, err)
			} else if claims.UserID != user.ID.Hex() {
				validationErrors++
				t.Logf("Token validation claim error %d: wrong user ID", i)
			}
		}

		if validationErrors > 0 {
			t.Errorf("Token validation errors: %d/%d", validationErrors, numTokenOps)
		}
	})
}

// TestUserService_DatabaseIntegrityConstraints tests database integrity
func TestUserService_DatabaseIntegrityConstraints(t *testing.T) {
	ctx := context.Background()

	t.Run("unique email constraint", func(t *testing.T) {
		email := "unique_" + generateTestSuffix() + "@example.com"

		// Create first user
		req1 := CreateUserRequest{
			UserName: "user1_" + generateTestSuffix(),
			Email:    email,
			Password: "password123",
		}

		_, err := testUserService.CreateUser(ctx, req1)
		if err != nil {
			t.Fatalf("Failed to create first user: %v", err)
		}

		// Try to create second user with same email
		req2 := CreateUserRequest{
			UserName: "user2_" + generateTestSuffix(),
			Email:    email, // Same email
			Password: "password456",
		}

		_, err = testUserService.CreateUser(ctx, req2)
		if err == nil {
			t.Error("Should not allow duplicate email")
		} else {
			t.Logf("Correctly prevented duplicate email: %v", err)
		}
	})

	t.Run("unique username constraint", func(t *testing.T) {
		username := "uniqueuser_" + generateTestSuffix()

		// Create first user
		req1 := CreateUserRequest{
			UserName: username,
			Email:    "unique1_" + generateTestSuffix() + "@example.com",
			Password: "password123",
		}

		_, err := testUserService.CreateUser(ctx, req1)
		if err != nil {
			t.Fatalf("Failed to create first user: %v", err)
		}

		// Try to create second user with same username
		req2 := CreateUserRequest{
			UserName: username, // Same username
			Email:    "unique2_" + generateTestSuffix() + "@example.com",
			Password: "password456",
		}

		_, err = testUserService.CreateUser(ctx, req2)
		if err == nil {
			t.Error("Should not allow duplicate username")
		} else {
			t.Logf("Correctly prevented duplicate username: %v", err)
		}
	})
}

// TestUserService_SecurityHeaders tests security-related functionality
func TestUserService_SecurityHeaders(t *testing.T) {
	ctx := context.Background()

	// Create a test user
	req := CreateUserRequest{
		UserName: "sectest_" + generateTestSuffix(),
		Email:    "security_" + generateTestSuffix() + "@example.com",
		Password: "password123",
	}

	user, err := testUserService.CreateUser(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	t.Run("password not exposed in user object", func(t *testing.T) {
		// Verify password is hashed and not the original
		if user.Password == req.Password {
			t.Error("Password should not be stored in plaintext")
		}

		// Verify password starts with bcrypt hash format
		if !strings.HasPrefix(user.Password, "$2") {
			t.Error("Password should be bcrypt hashed")
		}
	})

	t.Run("user ID generation", func(t *testing.T) {
		// Verify ObjectID is properly generated
		if user.ID.IsZero() {
			t.Error("User ID should be generated")
		}

		// Verify ObjectID is 24 characters hex string
		hexID := user.ID.Hex()
		if len(hexID) != 24 {
			t.Errorf("User ID hex should be 24 characters, got %d", len(hexID))
		}
	})

	t.Run("timestamp fields set", func(t *testing.T) {
		if user.CreatedAt.IsZero() {
			t.Error("CreatedAt should be set")
		}
		if user.UpdatedAt.IsZero() {
			t.Error("UpdatedAt should be set")
		}

		// Timestamps should be recent
		now := time.Now()
		if now.Sub(user.CreatedAt) > time.Minute {
			t.Error("CreatedAt should be recent")
		}
		if now.Sub(user.UpdatedAt) > time.Minute {
			t.Error("UpdatedAt should be recent")
		}
	})
}

