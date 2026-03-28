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

func UpdateAddon(c echo.Context) error {
	id := c.Param("id")
	var addon models.Addon
	if err := database.DB.First(&addon, id).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Addon not found"})
	}

	var req CreateAddonRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	if err := database.DB.Model(&addon).Updates(models.Addon{
		Name:     req.Name,
		Price:    req.Price,
		IsActive: req.IsActive,
	}).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not update addon"})
	}

	return c.JSON(http.StatusOK, addon)
}

func DeleteAddon(c echo.Context) error {
	id := c.Param("id")
	var addon models.Addon
	if err := database.DB.First(&addon, id).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Addon not found"})
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

