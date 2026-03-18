package handlers

import (
	"net/http"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

type UpdateProfileRequest struct {
	Name                  string   `json:"name"`
	Phone                 string   `json:"phone"`
	DOB                   string   `json:"dob"`
	Gender                string   `json:"gender"`
	Address               string   `json:"address"`
	EmergencyContactName  string   `json:"emergency_contact_name"`
	EmergencyContactPhone string   `json:"emergency_contact_phone"`
	BloodGroup            string   `json:"blood_group"`
	Height                *float64 `json:"height"`
	Weight                *float64 `json:"weight"`
	MedicalConditions     string   `json:"medical_conditions"`
}

func UpdateProfile(c echo.Context) error {
	userIDRaw := c.Get("user_id")
	if userIDRaw == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	userID := uint(userIDRaw.(float64))

	var req UpdateProfileRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	// Update only allowed fields
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.DOB != "" {
		user.DOB = req.DOB
	}
	if req.Gender != "" {
		user.Gender = req.Gender
	}
	if req.Address != "" {
		user.Address = req.Address
	}
	if req.EmergencyContactName != "" {
		user.EmergencyContactName = req.EmergencyContactName
	}
	if req.EmergencyContactPhone != "" {
		user.EmergencyContactPhone = req.EmergencyContactPhone
	}
	if req.BloodGroup != "" {
		user.BloodGroup = req.BloodGroup
	}
	if req.Height != nil {
		user.Height = req.Height
	}
	if req.Weight != nil {
		user.Weight = req.Weight
	}
	if req.MedicalConditions != "" {
		user.MedicalConditions = req.MedicalConditions
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not update profile"})
	}

	return c.JSON(http.StatusOK, user)
}
