package handlers

import (
	"net/http"
	"time"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

func GetDashboardStats(c echo.Context) error {
	gymIDRaw := c.Get("gym_id")
	role := c.Get("role").(string)
	
	var gymID uint
	hasGymID := false
	if gymIDRaw != nil {
		gymID = uint(gymIDRaw.(float64))
		hasGymID = true
	} else if role != "SuperAdmin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Missing gym_id"})
	}

	var totalMembers int64
	qMem := database.DB.Model(&models.User{}).Where("role = ?", "Member")
	if hasGymID {
		qMem = qMem.Where("gym_id = ?", gymID)
	}
	qMem.Count(&totalMembers)

	var activeMemberships int64
	qSub := database.DB.Model(&models.Subscription{}).
		Joins("JOIN users ON users.id = subscriptions.user_id").
		Where("subscriptions.status = ?", "Active")
	if hasGymID {
		qSub = qSub.Where("users.gym_id = ?", gymID)
	}
	qSub.Count(&activeMemberships)

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	
	var todaysAttendance int64
	qAtt := database.DB.Model(&models.Attendance{}).
		Joins("JOIN users ON users.id = attendances.user_id").
		Where("attendances.date = ?", today)
	if hasGymID {
		qAtt = qAtt.Where("users.gym_id = ?", gymID)
	}
	qAtt.Count(&todaysAttendance)

	// Revenue (sum of all paid payments)
	type Result struct {
		Total float64
	}
	var res Result
	qPay := database.DB.Model(&models.Payment{}).
		Select("sum(payments.amount) as total").
		Joins("JOIN subscriptions ON subscriptions.id = payments.subscription_id").
		Joins("JOIN users ON users.id = subscriptions.user_id").
		Where("payments.status = ?", "Paid")
	if hasGymID {
		qPay = qPay.Where("users.gym_id = ?", gymID)
	}
	qPay.Scan(&res)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"total_members":      totalMembers,
		"active_memberships": activeMemberships,
		"todays_attendance":  todaysAttendance,
		"total_revenue":      res.Total,
	})
}

func GetGyms(c echo.Context) error {
	var gyms []models.Gym
	if err := database.DB.Find(&gyms).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch gyms"})
	}
	return c.JSON(http.StatusOK, gyms)
}

func GetGym(c echo.Context) error {
	gymIdentifier := c.Param("identifier")
	var gym models.Gym

	// Try fetching by slug first, then by ID
	if err := database.DB.Where("slug = ?", gymIdentifier).First(&gym).Error; err != nil {
		if err := database.DB.First(&gym, gymIdentifier).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Gym not found"})
		}
	}

	return c.JSON(http.StatusOK, gym)
}

func AddGym(c echo.Context) error {
	var gym models.Gym
	if err := c.Bind(&gym); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})	
	}
	if err := database.DB.Create(&gym).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create gym"})
	}
	return c.JSON(http.StatusOK, gym)
}
