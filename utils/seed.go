package utils

import (
	"log"

	"gym-saas/database"
	"gym-saas/models"

	"golang.org/x/crypto/bcrypt"
)

func SeedSuperAdmin() {
	var count int64
	database.DB.Model(&models.User{}).Where("role = ?", "SuperAdmin").Count(&count)
	
	if count == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin := models.User{
			Name:         "Super Admin",
			Email:        "admin@gym.com",
			PasswordHash: string(hash),
			Role:         "SuperAdmin",
		}
		if err := database.DB.Create(&admin).Error; err != nil {
			log.Println("Error seeding admin:", err)
		} else {
			log.Println("Super Admin seeded: admin@gym.com / admin123")
		}
	}
}
