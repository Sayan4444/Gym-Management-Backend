package handlers

import (
	"net/http"
	"strconv"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

type AddonRequest struct {
	Name     *string  `json:"name"`
	Price    *float64 `json:"price"`
	IsActive *bool    `json:"is_active"`
}

func CreateAddon(c echo.Context) error {
	var req AddonRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	gymIDParam := c.Param("gymId")
	gymIDFromParam, err := strconv.ParseUint(gymIDParam, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Gym ID"})
	}

	gymIDRaw := c.Get("gym_id")
	if gymIDRaw == nil || uint(gymIDRaw.(float64)) != uint(gymIDFromParam) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to create addons for this gym"})
	}

	if req.Name == nil || req.Price == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Missing required fields"})
	}

	addon := models.Addon{
		GymID:    uint(gymIDFromParam),
		Name:     *req.Name,
		Price:    *req.Price,
		IsActive: true, // Defaulting to true as done in membership plan creation
	}

	if err := database.DB.Create(&addon).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not create addon"})
	}

	return c.JSON(http.StatusCreated, addon)
}

func UpdateAddon(c echo.Context) error {
	addonID := c.Param("addonId")
	gymIDParam := c.Param("gymId")

	gymIDFromParam, err := strconv.ParseUint(gymIDParam, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Gym ID"})
	}

	// 1. Fetch the existing addon first to check existence and ownership
	var addon models.Addon
	if err := database.DB.First(&addon, addonID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Addon not found"})
	}

	// Verify the addon actually belongs to the gym specified in the URL
	if addon.GymID != uint(gymIDFromParam) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Addon does not belong to the specified gym"})
	}

	// 2. Permission Checks using switch
	role := c.Get("role").(string)

	switch role {
	case "SuperAdmin":
		// SuperAdmin can update addons for any gym
	case "GymAdmin":
		// GymAdmin can only update addons belonging to their specific gym
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil || uint(gymIDRaw.(float64)) != addon.GymID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to update this addon"})
		}
	default:
		// Trainers or Members cannot update addons
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
	}

	// 3. Bind the incoming JSON to our pointer-based request struct
	var req AddonRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	// 4. Perform the update. GORM will only update fields that are not nil in the req struct.
	if err := database.DB.Model(&addon).Updates(req).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not update addon"})
	}

	// Fetch the fully updated addon from the DB to return to the client
	database.DB.First(&addon, addon.ID)
	return c.JSON(http.StatusOK, addon)
}

func DeleteAddon(c echo.Context) error {
	addonID := c.Param("addonId")
	gymIDParam := c.Param("gymId")

	gymIDFromParam, err := strconv.ParseUint(gymIDParam, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Gym ID"})
	}

	var addon models.Addon
	if err := database.DB.First(&addon, addonID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Addon not found"})
	}

	if addon.GymID != uint(gymIDFromParam) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Addon does not belong to the specified gym"})
	}

	gymIDRaw := c.Get("gym_id")
	if gymIDRaw == nil || uint(gymIDRaw.(float64)) != addon.GymID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to delete this addon"})
	}

	if err := database.DB.Delete(&addon).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not delete addon"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Addon deleted successfully"})
}

func GetAddonsByGym(c echo.Context) error {
	gymID := c.Param("gymId")

	var addons []models.Addon
	if err := database.DB.Where("gym_id = ? AND is_active = ?", gymID, true).Find(&addons).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not retrieve addons"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"count": len(addons), "addons": addons})
}

func GetAddons(c echo.Context) error {
	var addons []models.Addon
	role := c.Get("role").(string)

	switch role {
	case "SuperAdmin":
		// SuperAdmin can fetch all addons, or filter by a specific gym using ?gym_id=123
		gymIDStr := c.QueryParam("gym_id")

		if gymIDStr != "" {
			if err := database.DB.Where("gym_id = ?", gymIDStr).Find(&addons).Error; err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not fetch addons"})
			}
		} else {
			if err := database.DB.Find(&addons).Error; err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not fetch addons"})
			}
		}

	case "GymAdmin", "Trainer", "Member":
		// Standard roles can only view addons associated with their own gym
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied. No gym associated with your account."})
		}

		gymID := uint(gymIDRaw.(float64))
		if err := database.DB.Where("gym_id = ?", gymID).Find(&addons).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not fetch addons"})
		}

	default:
		// Catch-all for any unknown roles
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":  len(addons),
		"addons": addons,
	})
}