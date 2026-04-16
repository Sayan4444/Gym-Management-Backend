package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"gym-saas/database"
	"gym-saas/models"
	"gym-saas/utils"

	"github.com/labstack/echo/v4"
)

type GoogleLoginRequest struct {
	AccessToken string `json:"access_token"`
	GymId uint `json:"gym_id"`
}

func GoogleLogin(c echo.Context) error {
	var req GoogleLoginRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	// For implicit flow, the frontend sends the access_token in the credential field.
	// We need to fetch the user information from Google's UserInfo API.
	userInfoURL := "https://www.googleapis.com/oauth2/v3/userinfo?access_token=" + req.AccessToken
	resp, err := http.Get(userInfoURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("API Error (http.StatusUnauthorized): Invalid Google token")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid Google token"})
	}
	defer resp.Body.Close()

	var userInfo struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to decode user info"})
	}

	email := userInfo.Email
	if email == "" {
		log.Printf("API Error (http.StatusUnauthorized): Email not found in token")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Email not found in token"})
	}
	name := userInfo.Name

	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		log.Printf("Error: %v", err)
		// User not found, create them as Member

		user = models.User{
			Email: email,
			Role:  "Member",
			GymID: &req.GymId,
		}
		if name != "" {
			user.Name = name
		}
		if err := database.DB.Create(&user).Error; err != nil {
			log.Printf("Error: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create user"})
		}
	}

	token, err := utils.GenerateToken(user.ID, user.Role, user.GymID)
	if err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error generating token"})
	}

	// Set JWT as an HTTP-only secure cookie
	cookie := &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(24 * time.Hour / time.Second), // 24 hours
	}
	c.SetCookie(cookie)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"user": user,
	})
}

// Logout clears the auth cookie
func Logout(c echo.Context) error {
	cookie := &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // delete the cookie
	}
	c.SetCookie(cookie)
	return c.JSON(http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

// GetMe returns the currently authenticated user's data
func GetMe(c echo.Context) error {
	userIDRaw := c.Get("user_id")
	if userIDRaw == nil {
		log.Printf("API Error (http.StatusUnauthorized): Unauthorized")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	userID := uint(userIDRaw.(float64))

	var user models.User
	query := database.DB.Model(&models.User{})

	if includeParam := c.QueryParam("include"); includeParam != "" {
		includes := strings.SplitSeq(includeParam, ",")
		for relation := range includes {
			switch strings.ToLower(strings.TrimSpace(relation)) {
			case "gym":
				query = query.Preload("Gym")
			case "subscription":
				query = query.Preload("Subscription").Preload("Subscription.Plan")
			case "trainer":
				query = query.Preload("Trainer")
			case "	":
				query = query.Preload("WorkoutPlans")
			case "payments":
				query = query.Preload("Payments")
			case "user_addons":
				query = query.Preload("UserAddons").Preload("UserAddons.Addon")
			}
		}
	}

	if err := query.First(&user, userID).Error; err != nil {

		log.Printf("Error: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	return c.JSON(http.StatusOK, user)
}
