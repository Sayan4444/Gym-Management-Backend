package handlers

import (
	"net/http"
	"time"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

type DashboardStatsResponse struct {
	TotalMembers      int64   `json:"total_members"`
	TodaysAttendance  int64   `json:"todays_attendance"`
	ActiveMemberships int64   `json:"active_memberships"`
	ExpiringSoon      int64   `json:"expiring_soon"`
	TotalRevenue      float64 `json:"total_revenue"`
}

func GetSuperAdminDashboardStats(c echo.Context) error {
	var stats DashboardStatsResponse

	// Total Members (all users with role Member)
	database.DB.Model(&models.User{}).Where("role = ?", "Member").Count(&stats.TotalMembers)

	// Todays Attendance
	todayStart := time.Now().Truncate(24 * time.Hour)
	database.DB.Model(&models.Attendance{}).Where("date >= ?", todayStart).Count(&stats.TodaysAttendance)

	// Active Memberships
	database.DB.Model(&models.Subscription{}).Where("status = ?", "Active").Count(&stats.ActiveMemberships)

	// Expiring Soon (e.g., within next 7 days and Active)
	expiringDate := todayStart.AddDate(0, 0, 7)
	database.DB.Model(&models.Subscription{}).
		Where("status = ? AND end_date <= ?", "Active", expiringDate).
		Count(&stats.ExpiringSoon)

	// Total Revenue (all paid payments)
	var totalRevenue *float64
	database.DB.Model(&models.Payment{}).Where("status = ?", "Paid").Select("sum(amount)").Scan(&totalRevenue)
	if totalRevenue != nil {
		stats.TotalRevenue = *totalRevenue
	}

	return c.JSON(http.StatusOK, stats)
}

func GetAdminDashboardStats(c echo.Context) error {
	gymIDRaw := c.Get("gym_id")
	if gymIDRaw == nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Gym ID not found for admin"})
	}
	gymID := uint(gymIDRaw.(float64))

	var stats DashboardStatsResponse

	// Total Members for this gym
	database.DB.Model(&models.User{}).Where("role = ? AND gym_id = ?", "Member", gymID).Count(&stats.TotalMembers)

	// Todays Attendance for this gym
	todayStart := time.Now().Truncate(24 * time.Hour)
	database.DB.Table("attendances").
		Joins("JOIN users ON users.id = attendances.user_id").
		Where("attendances.date >= ? AND users.gym_id = ?", todayStart, gymID).
		Count(&stats.TodaysAttendance)

	// Active Memberships for this gym
	database.DB.Table("subscriptions").
		Joins("JOIN users ON users.id = subscriptions.user_id").
		Where("subscriptions.status = ? AND users.gym_id = ?", "Active", gymID).
		Count(&stats.ActiveMemberships)

	// Expiring Soon for this gym
	expiringDate := todayStart.AddDate(0, 0, 7)
	database.DB.Table("subscriptions").
		Joins("JOIN users ON users.id = subscriptions.user_id").
		Where("subscriptions.status = ? AND subscriptions.end_date <= ? AND users.gym_id = ?", "Active", expiringDate, gymID).
		Count(&stats.ExpiringSoon)

	// Total Revenue for this gym
	var totalRevenue *float64
	database.DB.Table("payments").
		Joins("JOIN users ON users.id = payments.user_id").
		Where("payments.status = ? AND users.gym_id = ?", "Paid", gymID).
		Select("sum(payments.amount)").Scan(&totalRevenue)
	if totalRevenue != nil {
		stats.TotalRevenue = *totalRevenue
	}

	return c.JSON(http.StatusOK, stats)
}
