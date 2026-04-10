package handlers

import (
	"errors"
	"log"
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

// this has the actual logic of creating the subscription
func AssignSubscriptionLogic(userID uint, planID uint) (*models.Subscription, *models.MembershipPlan, error) {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		log.Printf("Error: %v", err)
		return nil, nil, errors.New("User not found")
	}

	var plan models.MembershipPlan
	if err := database.DB.First(&plan, planID).Error; err != nil {
		log.Printf("Error: %v", err)
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

		log.Printf("Error: %v", err)
		return nil, nil, err
	}

	return &sub, &plan, nil
}

func AssignSubscription(c echo.Context) error {
	var req AssignSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	if roleRaw := c.Get("role"); roleRaw != nil {
		role := roleRaw.(string)
		if role == "GymAdmin" {
			gymIDRaw := c.Get("gym_id")
			if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
				log.Printf("API Error (http.StatusForbidden): You can only assign subscriptions to users in your gym")
				return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only assign subscriptions to users in your gym"})
			}
		}
	}

	sub, _, err := AssignSubscriptionLogic(req.UserID, req.PlanID)
	if err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, sub)
}

func GetSubscriptions(c echo.Context) error {
	var subs []models.Subscription

	// Extract role safely from context
	roleRaw := c.Get("role")
	if roleRaw == nil {
		log.Printf("API Error (http.StatusUnauthorized): Role missing from context")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Role missing from context"})
	}
	role := roleRaw.(string)

	// Initialize the base query
	query := database.DB.Model(&models.Subscription{}).Select("subscriptions.*")
	userIDParam := c.QueryParam("user_id")

	// Apply database-level filtering based on role
	switch role {
	case "SuperAdmin":
		// SuperAdmin sees everything globally.
		// Only filter by user_id if explicitly requested via query param.
		if userIDParam != "" {
			query = query.Where("subscriptions.user_id = ?", userIDParam)
		}

	case "GymAdmin", "Trainer":
		// GymAdmins and Trainers can only see subscriptions tied to their specific gym.
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil {
			log.Printf("API Error (http.StatusForbidden): Gym ID missing for this role")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Gym ID missing for this role"})
		}
		gymID := uint(gymIDRaw.(float64))

		// Optimize: Only JOIN the users table for roles that actually require checking the gym_id.
		query = query.Joins("JOIN users ON users.id = subscriptions.user_id").
			Where("users.gym_id = ?", gymID)

		// Allow admins/trainers to search for a specific user's subscriptions within their gym.
		if userIDParam != "" {
			query = query.Where("subscriptions.user_id = ?", userIDParam)
		}

	case "Member":
		// Members can strictly only view their own subscriptions.
		// Optimize/Secure: Ignore userIDParam entirely to prevent unauthorized access.
		userIDRaw := c.Get("user_id")
		if userIDRaw == nil {
			log.Printf("API Error (http.StatusUnauthorized): User ID missing from context")
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "User ID missing from context"})
		}

		// No JOIN required; directly query the user_id on the subscriptions table.
		query = query.Where("subscriptions.user_id = ?", uint(userIDRaw.(float64)))

	default:
		log.Printf("API Error (http.StatusForbidden): Invalid or unauthorized role")
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Invalid or unauthorized role"})
	}

	// Execute the finalized, optimized query
	if err := query.Find(&subs).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch subscriptions"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":         len(subs),
		"subscriptions": subs,
	})
}

type UpdateSubscriptionRequest struct {
	PlanID *uint   `json:"plan_id"`
	Status *string `json:"status"`
}

func UpdateSubscription(c echo.Context) error {
	id := c.Param("id")
	role := c.Get("role").(string)

	var sub models.Subscription
	if err := database.DB.First(&sub, id).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Subscription not found"})
	}

	var user models.User
	if err := database.DB.First(&user, sub.UserID).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "User not found"})
	}

	// Permission Checks using switch
	switch role {
	case "SuperAdmin":
		// SuperAdmin can update any subscription
	case "GymAdmin":
		// GymAdmin can only update subscriptions for users in their gym
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
			log.Printf("API Error (http.StatusForbidden): Access denied. You can only update subscriptions for users in your gym")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied. You can only update subscriptions for users in your gym"})
		}
	default:
		// Other roles cannot update subscription details
		log.Printf("API Error (http.StatusForbidden): Insufficient permissions")
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
	}

	var req UpdateSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	// Validate PlanID if it's being updated
	if req.PlanID != nil {
		var plan models.MembershipPlan
		if err := database.DB.First(&plan, *req.PlanID).Error; err != nil {
			log.Printf("Error: %v", err)
			return c.JSON(http.StatusNotFound, map[string]string{"error": "New plan not found"})
		}
		if user.GymID == nil || plan.GymID != *user.GymID {
			log.Printf("API Error (http.StatusBadRequest): User and new plan do not belong to the same gym")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "User and new plan do not belong to the same gym"})
		}
	}

	// Validate Status if it's being updated
	if req.Status != nil {
		if *req.Status == "Active" && sub.Status != "Active" {
			var existingActive models.Subscription
			if err := database.DB.Where("user_id = ? AND status = ? AND id != ?", sub.UserID, "Active", sub.ID).First(&existingActive).Error; err == nil {
				log.Printf("API Error (http.StatusConflict): User already has an active subscription")
				return c.JSON(http.StatusConflict, map[string]string{"error": "User already has an active subscription"})
			}
		}
	}

	// Use Updates with the request struct to properly handle partial updates via pointers
	if err := database.DB.Model(&sub).Updates(req).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update subscription"})
	}

	// Fetch the updated subscription to return the complete object
	database.DB.First(&sub, sub.ID)
	return c.JSON(http.StatusOK, sub)
}

func DeleteSubscription(c echo.Context) error {
	id := c.Param("id")
	var sub models.Subscription
	if err := database.DB.First(&sub, id).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Subscription not found"})
	}

	var user models.User
	if err := database.DB.First(&user, sub.UserID).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "User not found"})
	}

	role := c.Get("role").(string)
	if role == "GymAdmin" {
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
			log.Printf("API Error (http.StatusForbidden): You can only delete subscriptions for users in your gym")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only delete subscriptions for users in your gym"})
		}
	}

	if err := database.DB.Delete(&sub).Error; err != nil {

		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete subscription"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Subscription deleted successfully"})
}
