package handlers

import (
	"errors"
	"net/http"
	"time"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)



type AssignSubscriptionRequest struct {
	UserID uint `json:"user_id"`
	PlanID uint `json:"plan_id"`
}

func AssignSubscriptionLogic(userID uint, planID uint) (*models.Subscription, *models.MembershipPlan, error) {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return nil, nil, errors.New("User not found")
	}

	var plan models.MembershipPlan
	if err := database.DB.First(&plan, planID).Error; err != nil {
		return nil, nil, errors.New("Plan not found")
	}

	if user.GymID == nil || plan.GymID != *user.GymID {
		return nil, nil, errors.New("User and plan do not belong to the same gym")
	}

	var activeSub models.Subscription
	if err := database.DB.Where("user_id = ? AND status = ?", userID, "Active").First(&activeSub).Error; err == nil {
		return nil, nil, errors.New("user already has an active subscription")
	}

	startDate := time.Now()
	status := "Active"

	endDate := startDate.AddDate(0, plan.DurationMonths, 0)

	sub := models.Subscription{
		UserID:    userID,
		PlanID:    plan.ID,
		StartDate: startDate,
		EndDate:   endDate,
		Status:    status,
	}

	if err := database.DB.Create(&sub).Error; err != nil {
		return nil, nil, err
	}

	return &sub, &plan, nil
}

func CreateSubscription(userID uint, planID uint) (*models.Subscription, error) {
	sub, plan, err := AssignSubscriptionLogic(userID, planID)
	if err != nil {
		return nil, err
	}

	payment := models.Payment{
		UserID:      userID,
		Amount:      plan.Price,
		PaymentDate: sub.StartDate,
		Status:      "Paid",
	}
	database.DB.Create(&payment)

	go sendPaymentSuccessEmail(userID, plan.Price, plan.Name)

	return sub, nil
}

func AssignSubscription(c echo.Context) error {
	var req AssignSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	if roleRaw := c.Get("role"); roleRaw != nil {
		role := roleRaw.(string)
		if role == "GymAdmin" {
			gymIDRaw := c.Get("gym_id")
			if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only assign subscriptions to users in your gym"})
			}
		}
	}

	sub, err := CreateSubscription(req.UserID, req.PlanID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, sub)
}

func GetSubscriptions(c echo.Context) error {
	var subs []models.Subscription
	
	gymIDRaw := c.Get("gym_id")
	userIDRaw := c.Get("user_id")

	query := database.DB.Model(&models.Subscription{}).Select("subscriptions.*")

	// Filter by gym if it's a GymAdmin
	if gymIDRaw != nil {
		query = query.Joins("JOIN users ON users.id = subscriptions.user_id").
			Where("users.gym_id = ?", uint(gymIDRaw.(float64)))
	}

	// Filter by specific user if user_id is provided in query params (for admin viewing a member)
	userIDParam := c.QueryParam("user_id")
	if userIDParam != "" {
		query = query.Where("subscriptions.user_id = ?", userIDParam)
	} else if userIDRaw != nil && c.Get("role").(string) == "Member" {
		// If logged in as Member, they can only see their own
		query = query.Where("subscriptions.user_id = ?", uint(userIDRaw.(float64)))
	}

	if err := query.Find(&subs).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch subscriptions"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":         len(subs),
		"subscriptions": subs,
	})
}

type UpdateSubscriptionRequest struct {
	PlanID    uint       `json:"plan_id"`
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	Status    string     `json:"status"`
}

func UpdateSubscription(c echo.Context) error {
	id := c.Param("id")
	var sub models.Subscription
	if err := database.DB.First(&sub, id).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Subscription not found"})
	}

	var user models.User
	if err := database.DB.First(&user, sub.UserID).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "User not found"})
	}

	role := c.Get("role").(string)
	if role == "GymAdmin" {
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only update subscriptions for users in your gym"})
		}
	}

	var req UpdateSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if req.PlanID != 0 {
		var plan models.MembershipPlan
		if err := database.DB.First(&plan, req.PlanID).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "New plan not found"})
		}
		if user.GymID == nil || plan.GymID != *user.GymID {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "User and new plan do not belong to the same gym"})
		}
		sub.PlanID = req.PlanID
	}
	if req.StartDate != nil {
		sub.StartDate = *req.StartDate
	}
	if req.EndDate != nil {
		sub.EndDate = *req.EndDate
	}
	if req.Status != "" {
		if req.Status == "Active" && sub.Status != "Active" {
			var existingActive models.Subscription
			if err := database.DB.Where("user_id = ? AND status = ? AND id != ?", sub.UserID, "Active", sub.ID).First(&existingActive).Error; err == nil {
				return c.JSON(http.StatusConflict, map[string]string{"error": "user already has an active subscription"})
			}
		}
		sub.Status = req.Status
	}

	if err := database.DB.Save(&sub).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update subscription"})
	}

	return c.JSON(http.StatusOK, sub)
}

func DeleteSubscription(c echo.Context) error {
	id := c.Param("id")
	var sub models.Subscription
	if err := database.DB.First(&sub, id).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Subscription not found"})
	}

	var user models.User
	if err := database.DB.First(&user, sub.UserID).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "User not found"})
	}

	role := c.Get("role").(string)
	if role == "GymAdmin" {
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only delete subscriptions for users in your gym"})
		}
	}

	if err := database.DB.Delete(&sub).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete subscription"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Subscription deleted successfully"})
}