package config 

import (
	"fmt"
    "os"
    "strconv"
    "time"
    
    // Import godotenv for loading .env files
    _ "github.com/joho/godotenv/autoload"
)

type Config struct {
	Server ServerConfig `json:"server"`
	Database DatabaseConfig `json:"database"`
	JWT JWTConfig `json:"jwt"`
	Video VideoConfig `json:"video"`
	Security SecurityConfig `json:"security"`
}

type ServerConfig struct {
	Port         int           `json:"port"`
    Host         string        `json:"host"`
    ReadTimeout  time.Duration `json:"read_timeout"`
    WriteTimeout time.Duration `json:"write_timeout"`
    IdleTimeout  time.Duration `json:"idle_timeout"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
    Port     string `json:"port"`
    Name     string `json:"name"`
    Username string `json:"username"`
    Password string `json:"password"`
    URI      string `json:"uri"` // Full connection URI
}

type JWTConfig struct {
	SecretKey     string        `json:"secret_key"`
    Expiration    time.Duration `json:"expiration"`
    RefreshExpiration time.Duration `json:"refresh_expiration"`
}

type VideoConfig struct {
	UploadPath    string `json:"upload_path"`
    ProcessedPath string `json:"processed_path"`
    MaxFileSize   int64  `json:"max_file_size"` // in bytes
    AllowedTypes  []string `json:"allowed_types"`
}

type SecurityConfig struct {
	CORSOrigins []string `json:"cors_origins"`
    RateLimit   int      `json:"rate_limit"`
    RateWindow  time.Duration `json:"rate_window"`
}

//loads config from environment variables and .env file
func LoadConfig() (*Config, error) {
	config := &Config{}

	if err := config.loadServerConfig(); err != nil {
		return nil, fmt.Errorf("failed to load server config: %w", err)
	}

	if err := config.loadDatabaseConfig(); err != nil {
		return nil, fmt.Errorf("failed to load database config: %w", err)
	}

	if err := config.loadJWTConfig(); err != nil {
		return nil, fmt.Errorf("failed to load jwt config: %w", err)
	}

	if err := config.loadVideoConfig(); err != nil {
		return nil, fmt.Errorf("failed to load video config: %w", err)
	}

	if err := config.loadSecurityConfig(); err != nil {
		return nil, fmt.Errorf("failed to load security config: %w", err)
	}

	return config, nil

}

func (c *Config) loadServerConfig() error {
	portStr := getEnv("ENV", "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}

	c.Server = ServerConfig{
		Port: port,
		Host: getEnv("HOST", "0.0.0.0"),
		ReadTimeout: getDuration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout: getDuration("WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout: getDuration("IDLE_TIMEOUT", 10*time.Second),
	}
	return nil
}

func (c *Config) loadDatabaseConfig() error {
	c.Database = DatabaseConfig {
		Host:     getEnv("BLUEPRINT_DB_HOST", "localhost"),
        Port:     getEnv("BLUEPRINT_DB_PORT", "27017"),
        Name:     getEnv("DB_NAME", "streamflow"),
        Username: getEnv("DB_USERNAME", ""),
        Password: getEnv("DB_PASSWORD", ""),
	}

	if c.Database.Username != "" && c.Database.Password != ""{
		c.Database.URI = fmt.Sprintf("mongodb://%s:%s@%s:%s", c.Database.Username, c.Database.Password, c.Database.Host, c.Database.Port)
	} else {
		//no auth probs remove this 
		c.Database.URI = fmt.Sprintf("mongodb://%s:%s", c.Database.Host, c.Database.Port)
	}

	return nil
}

func (c *Config) loadJWTConfig() error {
	secretKey := getEnv("JWT_SECRET", "")
    if secretKey == "" {
        return fmt.Errorf("JWT_SECRET environment variable is required")
    }
    
    c.JWT = JWTConfig{
        SecretKey:        secretKey,
        Expiration:       getDurationEnv("JWT_EXPIRATION", 24*time.Hour),
        RefreshExpiration: getDurationEnv("JWT_REFRESH_EXPIRATION", 7*24*time.Hour),
    }

	return nil
}

func (c *Config) loadVideoConfig() error {
	c.Video = VideoConfig {
		UploadPath:    getEnv("VIDEO_UPLOAD_PATH", "storage/uploads"),
        ProcessedPath: getEnv("VIDEO_PROCESSED_PATH", "storage/processed"),
        MaxFileSize:   getInt64Env("VIDEO_MAX_FILE_SIZE", 100*1024*1024), // 100MB default
        AllowedTypes:  []string{"video/mp4", "video/avi", "video/mov", "video/mkv"},
	}
	return nil
}

func (c *Config) loadSecurityConfig() error {
	corsOriginsStr := getEnv("CORS_ORIGINS", "*")
	var corsOrigins []string
	if corsOriginStr != "*"{
		for _, origin := range strings.Split(corsOriginStr, ","){
			corsOrigins = append(corsOrigins, strings.TrimSpace(origin))
		}
	}
	else{
		corseOrigins = []string{"*"}
	}
	c.Security = SecurityConfig {
		CORSOrigins: corsOrigins,
		RateLimit: getIntEnv("RATE_LIMIT", 100),
		RateWindow: getDurationEnv("RATE_WINDOW", 1*time.Minute),
	}

	return nil
}

func getEnv(key string, defaultValue string) string {
	if value := os.Getenv(key); value != ""{
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != ""{
		intValue, err := strconv.Atoi(value); err == nil{
			return intValue
		}
	}
	return defaultValue
}

func getInt64Env(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != ""{
		intValue, err := strconv.ParseInt(value, 10, 64); err == nil{
			return intValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != ""{
		duration, err := time.ParseDuration(value); err == nil{
			return duration
		}
	}
	return defaultValue
}

func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Server.Port)
	}
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.JWT.SecretKey == "" {
		return fmt.Errorf("jwt secret key is required")
	}
	if c.Video.UploadPath == "" {
		return fmt.Errorf("video upload path is required")
	}
	
	return nil
}