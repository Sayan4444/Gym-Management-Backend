package handlers

import (
	"net/http"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

type CreateMemberRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
	PlanID   uint   `json:"plan_id"`
}

func CreateMember(c echo.Context) error {
	var req CreateMemberRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	
	gymIDRaw := c.Get("gym_id")
	var gymID *uint
	if gymIDRaw != nil {
		parsedGymID := uint(gymIDRaw.(float64))
		gymID = &parsedGymID
	}

	user := models.User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         "Member",
		GymID:        gymID,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not create user"})
	}

	profile := models.MemberProfile{
		UserID: user.ID,
		Phone:  req.Phone,
	}
	database.DB.Create(&profile)

	return c.JSON(http.StatusCreated, user)
}

func GetMembers(c echo.Context) error {
	var users []models.User
	
	gymIDRaw := c.Get("gym_id")
	query := database.DB.Where("role = ?", "Member").Preload("Gym")
	
	if gymIDRaw != nil {
		query = query.Where("gym_id = ?", uint(gymIDRaw.(float64)))
	}

	query.Find(&users)
	return c.JSON(http.StatusOK, users)
}
