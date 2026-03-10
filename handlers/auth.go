package handlers

import (
	"context"
	"net/http"

	"gym-saas/database"
	"gym-saas/models"
	"gym-saas/utils"

	"github.com/labstack/echo/v4"
	"google.golang.org/api/idtoken"
)

type GoogleLoginRequest struct {
	Credential string `json:"credential"`
}

func GoogleLogin(c echo.Context) error {
	var req GoogleLoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	payload, err := idtoken.Validate(context.Background(), req.Credential, "")
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid Google token"})
	}

	email, ok := payload.Claims["email"].(string)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Email not found in token"})
	}

	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		// User not found, create them as Member
		user = models.User{
			Email: email,
			Role:  "Member",
		}
		if name, ok := payload.Claims["name"].(string); ok {
			user.Name = name
		}
		if err := database.DB.Create(&user).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create user"})
		}
	}

	token, err := utils.GenerateToken(user.ID, user.Role, user.GymID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error generating token"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":     user.ID,
			"name":   user.Name,
			"email":  user.Email,
			"role":   user.Role,
			"gym_id": user.GymID,
		},
	})
}
