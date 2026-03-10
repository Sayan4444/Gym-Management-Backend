package handlers

import (
	"net/http"
	"time"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

type CreatePlanRequest struct {
	Name           string  `json:"name"`
	DurationMonths int     `json:"duration_months"`
	Price          float64 `json:"price"`
}

func CreatePlan(c echo.Context) error {
	var req CreatePlanRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	gymIDRaw := c.Get("gym_id")
	var gymID uint
	if gymIDRaw != nil {
		gymID = uint(gymIDRaw.(float64))
	}

	plan := models.MembershipPlan{
		GymID:          gymID,
		Name:           req.Name,
		DurationMonths: req.DurationMonths,
		Price:          req.Price,
		IsActive:       true,
	}

	if err := database.DB.Create(&plan).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not create plan"})
	}
	return c.JSON(http.StatusCreated, plan)
}

func GetPlans(c echo.Context) error {
	var plans []models.MembershipPlan
	gymIDRaw := c.Get("gym_id")
	if gymIDRaw != nil {
		database.DB.Where("gym_id = ?", uint(gymIDRaw.(float64))).Find(&plans)
	} else {
		database.DB.Find(&plans)
	}
	return c.JSON(http.StatusOK, plans)
}

type AssignSubscriptionRequest struct {
	UserID uint `json:"user_id"`
	PlanID uint `json:"plan_id"`
}

func AssignSubscription(c echo.Context) error {
	var req AssignSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	var plan models.MembershipPlan
	if err := database.DB.First(&plan, req.PlanID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Plan not found"})
	}

	startDate := time.Now()
	endDate := startDate.AddDate(0, plan.DurationMonths, 0)

	sub := models.Subscription{
		UserID:    req.UserID,
		PlanID:    plan.ID,
		StartDate: startDate,
		EndDate:   endDate,
		Status:    "Active",
	}

	database.DB.Create(&sub)

	payment := models.Payment{
		SubscriptionID: sub.ID,
		Amount:         plan.Price,
		PaymentDate:    startDate,
		Status:         "Paid",
	}
	database.DB.Create(&payment)

	return c.JSON(http.StatusCreated, sub)
}
