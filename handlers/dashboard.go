package handlers

import (
	"net/http"
	"time"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

type WeeklyAttendance struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

type MonthlyRevenue struct {
	Month   string  `json:"month"`
	Revenue float64 `json:"revenue"`
}

type DashboardStatsResponse struct {
	TotalMembers      int64              `json:"total_members"`
	TodaysAttendance  int64              `json:"todays_attendance"`
	ActiveMemberships int64              `json:"active_memberships"`
	ExpiringSoon      int64              `json:"expiring_soon"`
	TotalRevenue      float64            `json:"total_revenue"`
	WeeklyAttendance  []WeeklyAttendance `json:"weekly_attendance"`
	MonthlyRevenue    []MonthlyRevenue   `json:"monthly_revenue"`
}

func GetSuperAdminDashboardStats(c echo.Context) error {
	var stats DashboardStatsResponse

	database.DB.Model(&models.User{}).Where("role = ?", "Member").Count(&stats.TotalMembers)

	todayStart := time.Now().Truncate(24 * time.Hour)
	database.DB.Model(&models.Attendance{}).Where("date >= ?", todayStart).Count(&stats.TodaysAttendance)

	database.DB.Model(&models.Subscription{}).Where("status = ?", "Active").Count(&stats.ActiveMemberships)

	expiringDate := todayStart.AddDate(0, 0, 7)
	database.DB.Model(&models.Subscription{}).
		Where("status = ? AND end_date <= ?", "Active", expiringDate).
		Count(&stats.ExpiringSoon)

	var totalRevenue *float64
	database.DB.Model(&models.Payment{}).Where("status = ?", "Paid").Select("sum(amount)").Scan(&totalRevenue)
	if totalRevenue != nil {
		stats.TotalRevenue = *totalRevenue
	}

	// 1. Weekly Attendance (Current Week: Monday to Sunday)
	var weeklyAttendance []WeeklyAttendance
	
	// Determine how many days past Monday we are to find the start of the week
	weekday := int(todayStart.Weekday())
	if weekday == 0 {
		weekday = 7 // Shift Sunday from 0 to 7 to make Monday the start of the week
	}
	mondayStart := todayStart.AddDate(0, 0, -weekday+1)

	for i := 0; i < 7; i++ {
		dayStart := mondayStart.AddDate(0, 0, i)
		dayEnd := dayStart.AddDate(0, 0, 1)
		
		var count int64
		database.DB.Model(&models.Attendance{}).
			Where("date >= ? AND date < ?", dayStart, dayEnd).
			Count(&count)
			
		weeklyAttendance = append(weeklyAttendance, WeeklyAttendance{
			Day:   dayStart.Format("Mon"), // Formats as "Mon", "Tue", etc.
			Count: count,
		})
	}
	stats.WeeklyAttendance = weeklyAttendance

	// 2. Monthly Revenue (All 12 Months of the Current Year)
	var monthlyRevenue []MonthlyRevenue
	currentYear := todayStart.Year()

	for month := 1; month <= 12; month++ {
		mStart := time.Date(currentYear, time.Month(month), 1, 0, 0, 0, 0, todayStart.Location())
		mEnd := mStart.AddDate(0, 1, 0)
		
		var rev *float64
		database.DB.Model(&models.Payment{}).
			Where("status = ? AND created_at >= ? AND created_at < ?", "Paid", mStart, mEnd).
			Select("sum(amount)").Scan(&rev)
			
		var val float64
		if rev != nil {
			val = *rev
		}
		
		monthlyRevenue = append(monthlyRevenue, MonthlyRevenue{
			Month:   mStart.Format("Jan"), // Formats as "Jan", "Feb", etc.
			Revenue: val,
		})
	}
	stats.MonthlyRevenue = monthlyRevenue

	return c.JSON(http.StatusOK, stats)
}

func GetAdminDashboardStats(c echo.Context) error {
	gymIDRaw := c.Get("gym_id")
	if gymIDRaw == nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Gym ID not found for admin"})
	}
	gymID := uint(gymIDRaw.(float64))

	var stats DashboardStatsResponse

	database.DB.Model(&models.User{}).Where("role = ? AND gym_id = ?", "Member", gymID).Count(&stats.TotalMembers)

	todayStart := time.Now().Truncate(24 * time.Hour)
	database.DB.Table("attendances").
		Joins("JOIN users ON users.id = attendances.user_id").
		Where("attendances.date >= ? AND users.gym_id = ?", todayStart, gymID).
		Count(&stats.TodaysAttendance)

	database.DB.Table("subscriptions").
		Joins("JOIN users ON users.id = subscriptions.user_id").
		Where("subscriptions.status = ? AND users.gym_id = ?", "Active", gymID).
		Count(&stats.ActiveMemberships)

	expiringDate := todayStart.AddDate(0, 0, 7)
	database.DB.Table("subscriptions").
		Joins("JOIN users ON users.id = subscriptions.user_id").
		Where("subscriptions.status = ? AND subscriptions.end_date <= ? AND users.gym_id = ?", "Active", expiringDate, gymID).
		Count(&stats.ExpiringSoon)

	var totalRevenue *float64
	database.DB.Table("payments").
		Joins("JOIN users ON users.id = payments.user_id").
		Where("payments.status = ? AND users.gym_id = ?", "Paid", gymID).
		Select("sum(payments.amount)").Scan(&totalRevenue)
	if totalRevenue != nil {
		stats.TotalRevenue = *totalRevenue
	}

	var weeklyAttendance []WeeklyAttendance
	for i := 6; i >= 0; i-- {
		dayStart := todayStart.AddDate(0, 0, -i)
		dayEnd := dayStart.AddDate(0, 0, 1)
		var count int64
		database.DB.Table("attendances").
			Joins("JOIN users ON users.id = attendances.user_id").
			Where("attendances.date >= ? AND attendances.date < ? AND users.gym_id = ?", dayStart, dayEnd, gymID).
			Count(&count)
		weeklyAttendance = append(weeklyAttendance, WeeklyAttendance{Day: dayStart.Format("Mon"), Count: count})
	}
	stats.WeeklyAttendance = weeklyAttendance

	var monthlyRevenue []MonthlyRevenue
	for i := 2; i >= 0; i-- {
		mStart := time.Date(todayStart.Year(), todayStart.Month()-time.Month(i), 1, 0, 0, 0, 0, todayStart.Location())
		mEnd := mStart.AddDate(0, 1, 0)
		var rev *float64
		database.DB.Table("payments").
			Joins("JOIN users ON users.id = payments.user_id").
			Where("payments.status = ? AND payments.created_at >= ? AND payments.created_at < ? AND users.gym_id = ?", "Paid", mStart, mEnd, gymID).
			Select("sum(payments.amount)").Scan(&rev)
		var val float64
		if rev != nil {
			val = *rev
		}
		monthlyRevenue = append(monthlyRevenue, MonthlyRevenue{Month: mStart.Format("Jan '06"), Revenue: val})
	}
	stats.MonthlyRevenue = monthlyRevenue

	return c.JSON(http.StatusOK, stats)
}
