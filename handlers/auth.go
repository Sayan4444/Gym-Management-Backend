package handlers

import (
	"encoding/json"
	"net/http"

	"gym-saas/database"
	"gym-saas/models"
	"gym-saas/utils"

	"github.com/labstack/echo/v4"
)

type GoogleLoginRequest struct {
	Credential string `json:"credential"`
}

func GoogleLogin(c echo.Context) error {
	var req GoogleLoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	// For implicit flow, the frontend sends the access_token in the credential field.
	// We need to fetch the user information from Google's UserInfo API.
	userInfoURL := "https://www.googleapis.com/oauth2/v3/userinfo?access_token=" + req.Credential
	resp, err := http.Get(userInfoURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid Google token"})
	}
	defer resp.Body.Close()

	var userInfo struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to decode user info"})
	}

	email := userInfo.Email
	if email == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Email not found in token"})
	}
	name := userInfo.Name

	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		// User not found, create them as Member
		user = models.User{
			Email: email,
			Role:  "Member",
		}
		if name != "" {
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
