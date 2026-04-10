package handlers

import (
	"errors"
	"log"
	"net/http"
	"time"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

type AssignUserAddonRequest struct {
	UserID  uint `json:"user_id"`
	AddonID uint `json:"addon_id"`
}

func AssignUserAddonLogic(userID uint, addonID uint) (*models.UserAddon, *models.Addon, error) {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		log.Printf("Error: %v", err)
		return nil, nil, errors.New("User not found")
	}

	var addon models.Addon
	if err := database.DB.First(&addon, addonID).Error; err != nil {
		log.Printf("Error: %v", err)
		return nil, nil, errors.New("Addon not found")
	}

	if user.GymID == nil || addon.GymID != *user.GymID {
		return nil, nil, errors.New("User and addon do not belong to the same gym")
	}

	purchasedAt := time.Now()

	userAddon := models.UserAddon{
		UserID:      userID,
		AddonID:     addon.ID,
		PurchasedAt: purchasedAt,
	}

	if err := database.DB.Create(&userAddon).Error; err != nil {

		log.Printf("Error: %v", err)
		return nil, nil, err
	}

	return &userAddon, &addon, nil
}

func AssignUserAddon(c echo.Context) error {
	var req AssignUserAddonRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	if roleRaw := c.Get("role"); roleRaw != nil {
		role := roleRaw.(string)
		if role == "GymAdmin" {
			gymIDRaw := c.Get("gym_id")
			if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
				log.Printf("API Error (http.StatusForbidden): You can only assign addons to users in your gym")
				return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only assign addons to users in your gym"})
			}
		}
	}

	userAddon, _, err := AssignUserAddonLogic(req.UserID, req.AddonID)
	if err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, userAddon)
}

func GetUserAddons(c echo.Context) error {
	var userAddons []models.UserAddon

	// Extract role safely from context
	roleRaw := c.Get("role")
	if roleRaw == nil {
		log.Printf("API Error (http.StatusUnauthorized): Role missing from context")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Role missing from context"})
	}
	role := roleRaw.(string)

	// Initialize the base query
	query := database.DB.Model(&models.UserAddon{}).Select("user_addons.*")
	userIDParam := c.QueryParam("user_id")

	// Apply database-level filtering based on role
	switch role {
	case "SuperAdmin":
		// SuperAdmin sees everything globally.
		// Only filter by user_id if explicitly requested via query param.
		if userIDParam != "" {
			query = query.Where("user_addons.user_id = ?", userIDParam)
		}

	case "GymAdmin", "Trainer":
		// GymAdmins and Trainers can only see addons tied to their specific gym.
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil {
			log.Printf("API Error (http.StatusForbidden): Gym ID missing for this role")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Gym ID missing for this role"})
		}
		gymID := uint(gymIDRaw.(float64))

		// Optimize: Only JOIN the users table for roles that actually require checking the gym_id.
		query = query.Joins("JOIN users ON users.id = user_addons.user_id").
			Where("users.gym_id = ?", gymID)

		// Allow admins/trainers to search for a specific user's addons within their gym.
		if userIDParam != "" {
			query = query.Where("user_addons.user_id = ?", userIDParam)
		}

	case "Member":
		// Members can strictly only view their own addons.
		// Optimize/Secure: Ignore userIDParam entirely to prevent unauthorized access.
		userIDRaw := c.Get("user_id")
		if userIDRaw == nil {
			log.Printf("API Error (http.StatusUnauthorized): User ID missing from context")
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "User ID missing from context"})
		}

		// No JOIN required; directly query the user_id on the user_addons table.
		query = query.Where("user_addons.user_id = ?", uint(userIDRaw.(float64)))

	default:
		log.Printf("API Error (http.StatusForbidden): Invalid or unauthorized role")
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Invalid or unauthorized role"})
	}

	// Execute the finalized, optimized query
	if err := query.Find(&userAddons).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch user addons"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":       len(userAddons),
		"user_addons": userAddons,
	})
}

// Struct modified to use pointers for partial updates via GORM
type UpdateUserAddonRequest struct {
	AddonID *uint `json:"addon_id"`
}

func UpdateUserAddon(c echo.Context) error {
	id := c.Param("id")
	role := c.Get("role").(string)

	var userAddon models.UserAddon
	if err := database.DB.First(&userAddon, id).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User addon not found"})
	}

	var user models.User
	if err := database.DB.First(&user, userAddon.UserID).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "User not found"})
	}

	// Permission Checks using switch
	switch role {
	case "SuperAdmin":
		// SuperAdmin can update any user addon
	case "GymAdmin":
		// GymAdmin can only update addons for users in their gym
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
			log.Printf("API Error (http.StatusForbidden): Access denied. You can only update addons for users in your gym")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied. You can only update addons for users in your gym"})
		}
	default:
		// Other roles cannot update addon details
		log.Printf("API Error (http.StatusForbidden): Insufficient permissions")
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
	}

	var req UpdateUserAddonRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	// Validate AddonID if it's being updated
	if req.AddonID != nil {
		var addon models.Addon
		if err := database.DB.First(&addon, *req.AddonID).Error; err != nil {
			log.Printf("Error: %v", err)
			return c.JSON(http.StatusNotFound, map[string]string{"error": "New addon not found"})
		}
		if user.GymID == nil || addon.GymID != *user.GymID {
			log.Printf("API Error (http.StatusBadRequest): User and new addon do not belong to the same gym")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "User and new addon do not belong to the same gym"})
		}
	}

	// Use Updates with the request struct to properly handle partial updates via pointers
	if err := database.DB.Model(&userAddon).Updates(req).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update user addon"})
	}

	// Fetch the updated user addon to return the complete object
	database.DB.First(&userAddon, userAddon.ID)
	return c.JSON(http.StatusOK, userAddon)
}

func DeleteUserAddon(c echo.Context) error {
	id := c.Param("id")
	var userAddon models.UserAddon
	if err := database.DB.First(&userAddon, id).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User addon not found"})
	}

	var user models.User
	if err := database.DB.First(&user, userAddon.UserID).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "User not found"})
	}

	role := c.Get("role").(string)
	if role == "GymAdmin" {
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
			log.Printf("API Error (http.StatusForbidden): You can only delete addons for users in your gym")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only delete addons for users in your gym"})
		}
	}

	if err := database.DB.Delete(&userAddon).Error; err != nil {

		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete user addon"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "User addon deleted successfully"})
}
