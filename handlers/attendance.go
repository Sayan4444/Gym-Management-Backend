package handlers

import (
	"net/http"
	"time"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

type LogAttendanceRequest struct {
	UserID uint   `json:"user_id"`
	Source string `json:"source"`
}

func LogAttendance(c echo.Context) error {
	var req LogAttendanceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var existing models.Attendance
	err := database.DB.Where("user_id = ? AND date = ?", req.UserID, today).First(&existing).Error
	
	if err == nil {
		// Already checked in, check out
		existing.TimeOut = &now
		database.DB.Save(&existing)
		return c.JSON(http.StatusOK, map[string]string{"message": "Checked out successfully"})
	}

	att := models.Attendance{
		UserID: req.UserID,
		Date:   today,
		TimeIn: now,
		Source: req.Source,
	}

	if err := database.DB.Create(&att).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not log attendance"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Checked in successfully"})
}
