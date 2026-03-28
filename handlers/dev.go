package handlers

import (
	"net/http"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

func ChangeRole(c echo.Context) error {
	var req struct {
		UserID uint   `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	user.Role = req.Role
	if err := database.DB.Save(&user).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update role"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Role updated successfully"})
}