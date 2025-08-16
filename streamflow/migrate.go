package main

import (
	"context"
	"log"
	"streamflow/internal/database"
	"streamflow/internal/video"
)

func main() {
	log.Println("Starting video field migration...")
	
	// Connect to database
	db := database.New()
	defer db.Close()
	
	// Create video service
	videoService := video.NewVideoService(db.GetDatabase())
	
	// Run field migration
	ctx := context.Background()
	err := videoService.MigrateVideoFieldNames(ctx)
	if err != nil {
		log.Fatalf("Failed to migrate video fields: %v", err)
	}
	
	log.Println("Video field migration completed successfully!")
}