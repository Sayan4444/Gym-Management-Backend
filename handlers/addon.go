package handlers

import (
	"net/http"
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

	gymIDRaw := c.Get("gym_id")
	if gymIDRaw == nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Missing gym_id"})
	}
	gymID := uint(gymIDRaw.(float64))

	addon := models.Addon{
		GymID:    gymID,
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

type BuyAddonRequest struct {
	UserID  uint `json:"user_id"`
	AddonID uint `json:"addon_id"`
}

func BuyAddon(c echo.Context) error {
	var req BuyAddonRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	var addon models.Addon
	if err := database.DB.First(&addon, req.AddonID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Addon not found"})
	}

	// Create payment
	payment := models.Payment{
		UserID:      req.UserID,
		Amount:      addon.Price,
		PaymentDate: time.Now(),
		Status:      "Paid",
		PaymentFor:  "Add-On",
	}

	if err := database.DB.Create(&payment).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to process payment"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Addon purchased successfully",
		"payment": payment,
	})
}
