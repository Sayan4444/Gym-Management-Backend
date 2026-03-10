package main

import (
	"fmt"
	"log"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load("../../.env"); err != nil {
		if err = godotenv.Load("../.env"); err != nil {
			if err = godotenv.Load(".env"); err != nil {
				log.Println("Could not load .env file, relying on environment variables. Error:", err)
			}
		}
	}

	// Initialize the database connection.
	database.InitDB()

	db := database.DB
	if db == nil {
		log.Fatal("Database connection failed")
	}

	fmt.Println("Removing models from database...")

	// Drop all tables to completely remove the models
	err := db.Migrator().DropTable(
		&models.WorkoutPlan{},
		&models.Attendance{},
		&models.Payment{},
		&models.Subscription{},
		&models.MembershipPlan{},
		&models.User{},
		&models.Gym{},
	)
	if err != nil {
		log.Fatalf("Failed to remove models (drop tables): %v", err)
	}

	fmt.Println("Database completely cleared successfully!")
}
