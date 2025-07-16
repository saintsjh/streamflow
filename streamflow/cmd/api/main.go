package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "streamflow/internal/config"
    "streamflow/internal/server"
    "syscall"
    "time"
)

func gracefulShutdown(fiberServer *server.FiberServer, done chan bool) {
    // Create context that listens for the interrupt signal from the OS.
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    // Listen for the interrupt signal.
    <-ctx.Done()

    log.Println("shutting down gracefully, press Ctrl+C again to force")
    stop() // Allow Ctrl+C to force shutdown

    // The context is used to inform the server it has 5 seconds to finish
    // the request it is currently handling
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := fiberServer.ShutdownWithContext(ctx); err != nil {
        log.Printf("Server forced to shutdown with error: %v", err)
    }

    log.Println("Server exiting")

    // Notify the main goroutine that the shutdown is complete
    done <- true
}

func main() {
    // Load configuration from environment variables
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }
    
    // Validate the configuration
    if err := cfg.Validate(); err != nil {
        log.Fatalf("Invalid configuration: %v", err)
    }
    
    // Log configuration (be careful not to log secrets in production)
    log.Printf("Server starting on %s:%d", cfg.Server.Host, cfg.Server.Port)
    log.Printf("Database: %s", cfg.Database.Host)
    log.Printf("Video upload path: %s", cfg.Video.UploadPath)

    // Create server with configuration
    server := server.New(cfg)

    server.RegisterFiberRoutes()

    // Create a done channel to signal when the shutdown is complete
    done := make(chan bool, 1)

    go func() {
        // Use configuration for server address
        addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
        err := server.Listen(addr)
        if err != nil {
            panic(fmt.Sprintf("http server error: %s", err))
        }
    }()

    // Run graceful shutdown in a separate goroutine
    go gracefulShutdown(server, done)

    // Wait for the graceful shutdown to complete
    <-done
    log.Println("Graceful shutdown complete.")
}