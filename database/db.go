package database

import (
	"fmt"
	"log"
	"os"

	"gym-saas/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	port := os.Getenv("DB_PORT")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC", host, user, password, dbname, port)
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(
		&models.Gym{},
		&models.User{},
		&models.MembershipPlan{},
		&models.Subscription{},
		&models.Payment{},
		&models.Attendance{},
		&models.WorkoutPlan{},
		&models.Addon{},
		&models.UserAddon{},
		&models.GymQRToken{},
	)
	if err != nil {
		log.Fatalf("Failed to auto-migrate database: %v", err)
	}

	DB = db
	log.Println("Database connection established and migrated")
}
