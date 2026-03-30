package handlers

import (
	"errors"
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
		return nil, nil, errors.New("User not found")
	}

	var addon models.Addon
	if err := database.DB.First(&addon, addonID).Error; err != nil {
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
		return nil, nil, err
	}

	return &userAddon, &addon, nil
}

func AssignUserAddon(c echo.Context) error {
	var req AssignUserAddonRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	if roleRaw := c.Get("role"); roleRaw != nil {
		role := roleRaw.(string)
		if role == "GymAdmin" {
			gymIDRaw := c.Get("gym_id")
			if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only assign addons to users in your gym"})
			}
		}
	}

	userAddon, _, err := AssignUserAddonLogic(req.UserID, req.AddonID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, userAddon)
}

func GetUserAddons(c echo.Context) error {
	var userAddons []models.UserAddon
	
	gymIDRaw := c.Get("gym_id")
	userIDRaw := c.Get("user_id")

	query := database.DB.Model(&models.UserAddon{}).Select("user_addons.*")

	// Filter by gym if it's a GymAdmin
	if gymIDRaw != nil {
		query = query.Joins("JOIN users ON users.id = user_addons.user_id").
			Where("users.gym_id = ?", uint(gymIDRaw.(float64)))
	}

	// Filter by specific user if user_id is provided in query params
	userIDParam := c.QueryParam("user_id")
	if userIDParam != "" {
		query = query.Where("user_addons.user_id = ?", userIDParam)
	} else if userIDRaw != nil && c.Get("role").(string) == "Member" {
		// If logged in as Member, they can only see their own
		query = query.Where("user_addons.user_id = ?", uint(userIDRaw.(float64)))
	}

	if err := query.Find(&userAddons).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch user addons"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":       len(userAddons),
		"user_addons": userAddons,
	})
}

type UpdateUserAddonRequest struct {
	AddonID     uint       `json:"addon_id"`
	PurchasedAt *time.Time `json:"purchased_at"`
}

func UpdateUserAddon(c echo.Context) error {
	id := c.Param("id")
	var userAddon models.UserAddon
	if err := database.DB.First(&userAddon, id).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User addon not found"})
	}

	var user models.User
	if err := database.DB.First(&user, userAddon.UserID).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "User not found"})
	}

	role := c.Get("role").(string)
	if role == "GymAdmin" {
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only update addons for users in your gym"})
		}
	}

	var req UpdateUserAddonRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if req.AddonID != 0 {
		var addon models.Addon
		if err := database.DB.First(&addon, req.AddonID).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "New addon not found"})
		}
		if user.GymID == nil || addon.GymID != *user.GymID {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "User and new addon do not belong to the same gym"})
		}
		userAddon.AddonID = req.AddonID
	}
	if req.PurchasedAt != nil {
		userAddon.PurchasedAt = *req.PurchasedAt
	}

	if err := database.DB.Save(&userAddon).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update user addon"})
	}

	return c.JSON(http.StatusOK, userAddon)
}

func DeleteUserAddon(c echo.Context) error {
	id := c.Param("id")
	var userAddon models.UserAddon
	if err := database.DB.First(&userAddon, id).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User addon not found"})
	}

	var user models.User
	if err := database.DB.First(&user, userAddon.UserID).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "User not found"})
	}

	role := c.Get("role").(string)
	if role == "GymAdmin" {
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only delete addons for users in your gym"})
		}
	}

	if err := database.DB.Delete(&userAddon).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete user addon"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "User addon deleted successfully"})
}
