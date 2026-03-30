package handlers

import (
	"net/http"
	"strconv"
	"time"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

type CreateAddonRequest struct {
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	IsActive bool    `json:"is_active"`
}

func CreateAddon(c echo.Context) error {
	var req CreateAddonRequest
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

	addon := models.Addon{
		GymID:    uint(gymIDFromParam),
		Name:     req.Name,
		Price:    req.Price,
		IsActive: req.IsActive,
	}

	if err := database.DB.Create(&addon).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not create addon"})
	}
	return c.JSON(http.StatusCreated, addon)
}

func GetAddons(c echo.Context) error {
	var addons []models.Addon
	gymIDRaw := c.Get("gym_id")
	if gymIDRaw != nil {	
		database.DB.Where("gym_id = ?", uint(gymIDRaw.(float64))).Find(&addons)
	} else {
		// SuperAdmin might request all addons? Or maybe we require a gym_id query param
		gymIDStr := c.QueryParam("gym_id")
		if gymIDStr != "" {
			database.DB.Where("gym_id = ?", gymIDStr).Find(&addons)
		} else {
			database.DB.Find(&addons)
		}
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":  len(addons),
		"addons": addons,
	})
}

func GetAddonsByGym(c echo.Context) error {
	gymID := c.Param("gymId")
	
	var addons []models.Addon
	if err := database.DB.Where("gym_id = ? AND is_active = ?", gymID, true).Find(&addons).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not fetch addons"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":  len(addons),
		"addons": addons,
	})
}

type BuyAddonRequest struct {
	UserID  uint `json:"user_id"`
	AddonID uint `json:"addon_id"`
}

func UpdateAddon(c echo.Context) error {
	addonID := c.Param("addonId")
	gymIDParam := c.Param("gymId")

	gymIDFromParam, err := strconv.ParseUint(gymIDParam, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Gym ID"})
	}

	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	updateData := make(map[string]interface{})
	if val, ok := req["name"]; ok {
		updateData["name"] = val
	}
	if val, ok := req["price"]; ok {
		updateData["price"] = val
	}
	if val, ok := req["is_active"]; ok {
		updateData["is_active"] = val
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
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to update this addon"})
	}

	if err := database.DB.Model(&addon).Updates(updateData).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not update addon"})
	}

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

// AssignAddonLogic records a UserAddon purchase for the given user, addon, and payment.
// Returns the created UserAddon and an error (if any).
func AssignAddonLogic(userID uint, addonID uint, paymentID uint) (*models.UserAddon, error) {
	userAddon := models.UserAddon{
		UserID:      userID,
		AddonID:     addonID,
		PaymentID:   paymentID,
		PurchasedAt: time.Now(),
	}

	if err := database.DB.Create(&userAddon).Error; err != nil {
		return nil, err
	}

	return &userAddon, nil
}

