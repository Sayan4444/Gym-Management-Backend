package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load("../.env"); err != nil {
		if err = godotenv.Load(".env"); err != nil {
			log.Println("Could not load .env file, relying on environment variables. Error:", err)
		}
	}

	// Initialize the database connection.
	// We'll set the environment variables manually if they are empty
	database.InitDB()

	db := database.DB
	if db == nil {
		log.Fatal("Database connection failed")
	}

	fmt.Println("Ensuring tables are created...")
	err := db.AutoMigrate(
		&models.Gym{},
		&models.User{},
		&models.MembershipPlan{},
		&models.Subscription{},
		&models.Payment{},
		&models.Attendance{},
		&models.WorkoutPlan{},
		&models.WorkoutExercise{},
	)
	if err != nil {
		log.Fatalf("Failed to auto-migrate database: %v", err)
	}

	fmt.Println("Seeding database with fake data...")

	// Seed 5 Gyms
	var gyms []models.Gym
	for i := 1; i <= 5; i++ {
		gym := models.Gym{
			Name:    fmt.Sprintf("Fake Gym %d", i),
			Address: fmt.Sprintf("%d Fake St, City %d", i*10, i),
		}
		db.Create(&gym)
		gyms = append(gyms, gym)
	}
	fmt.Println("Seeded 5 Gyms.")

	var users []models.User

	// Seed 1 Super Admin Explicitly
	superAdmin := models.User{
		Name:        "Super Admin",
		Email:       "superadmin@example.com",
		Phone:       "+10000000000",
		DOB:         "1980-01-01",
		Gender:      "Other",
		BiometricID: "BIO0",
		Role:        "SuperAdmin",
		GymID:       nil, // SuperAdmin may not be tied to a specific gym
	}
	db.Create(&superAdmin)
	users = append(users, superAdmin)
	fmt.Println("Seeded Super Admin explicitly.")

	// Seed remaining Users
	roles := []string{"GymAdmin", "Trainer", "Member", "Member"}
	for i := 1; i <= 4; i++ {
		user := models.User{
			Name:        fmt.Sprintf("Fake User %d", i),
			Email:       fmt.Sprintf("user%d@example.com", i),
			Phone:       fmt.Sprintf("+1000000000%d", i),
			DOB:         "1990-01-01",
			Gender:      "Other",
			BiometricID: fmt.Sprintf("BIO%d", i),
			Role:        roles[i-1],
			GymID:       &gyms[i%len(gyms)].ID,
		}
		db.Create(&user)
		users = append(users, user)
	}
	fmt.Println("Seeded 4 other Users.")

	// Seed 5 Membership Plans
	var plans []models.MembershipPlan
	for i := 1; i <= 5; i++ {
		plan := models.MembershipPlan{
			GymID:          gyms[i%len(gyms)].ID,
			Name:           fmt.Sprintf("Plan %d", i),
			Price:          float64(i * 100),
			DurationMonths: i,
			IsActive:       true,
		}
		db.Create(&plan)
		plans = append(plans, plan)
	}
	fmt.Println("Seeded 5 MembershipPlans.")

	// Seed 5 Subscriptions
	for i := 1; i <= 5; i++ {
		sub := models.Subscription{
			UserID:    users[i%len(users)].ID,
			PlanID:    plans[i%len(plans)].ID,
			StartDate: time.Now(),
			EndDate:   time.Now().AddDate(0, plans[i%len(plans)].DurationMonths, 0),
			Status:    "Active",
		}
		db.Create(&sub)
	}
	fmt.Println("Seeded 5 Subscriptions.")

	// Seed 5 Payments
	for i := 1; i <= 5; i++ {
		payment := models.Payment{
			UserID:      users[i%len(users)].ID,
			Amount:      float64(i * 100),
			PaymentDate: time.Now(),
			Status:      "Paid",
		}
		db.Create(&payment)
	}
	fmt.Println("Seeded 5 Payments.")

	// Seed 5 Attendances
	for i := 1; i <= 5; i++ {
		timeIn := time.Now().Add(-time.Duration(rand.Intn(5)) * time.Hour)
		timeOut := timeIn.Add(time.Duration(1) * time.Hour)
		attendance := models.Attendance{
			UserID:  users[i%len(users)].ID,
			Date:    time.Now().Truncate(24 * time.Hour),
			TimeIn:  timeIn,
			TimeOut: &timeOut,
			Source:  "Manual",
		}
		db.Create(&attendance)
	}
	fmt.Println("Seeded 5 Attendances.")

	// Seed 5 WorkoutPlans
	for i := 1; i <= 5; i++ {
		workoutPlan := models.WorkoutPlan{
			GymID:     gyms[i%len(gyms)].ID,
			TrainerID: users[2].ID, // index 2 is Trainer
			MemberID:  users[3].ID, // index 3 is Member
			Title:     fmt.Sprintf("Workout Plan %d", i),
			Exercises: []models.WorkoutExercise{
				{Name: fmt.Sprintf("%d Push-ups", i*10)},
				{Name: fmt.Sprintf("%d Squats X 3", i*5)},
			},
		}
		db.Create(&workoutPlan)
	}
	fmt.Println("Seeded 5 WorkoutPlans.")

	fmt.Println("Seed completed successfully!")
}
