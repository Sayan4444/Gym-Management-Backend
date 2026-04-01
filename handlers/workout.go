package handlers

import (
	"net/http"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

type CreateWorkoutPlanRequest struct {
	MemberID    uint   `json:"member_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// CreateWorkoutPlan - Only the trainer assigned to the member can create a workout plan.
func CreateWorkoutPlan(c echo.Context) error {
	var req CreateWorkoutPlanRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	trainerIDRaw := c.Get("user_id")
	if trainerIDRaw == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}
	trainerID := uint(trainerIDRaw.(float64))

	gymIDRaw := c.Get("gym_id")
	if gymIDRaw == nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Missing gym_id"})
	}
	gymID := uint(gymIDRaw.(float64))

	// Verify the target member exists and has this trainer assigned
	var member models.User
	if err := database.DB.First(&member, req.MemberID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Member not found"})
	}

	if member.TrainerID == nil || *member.TrainerID != trainerID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You are not the assigned trainer for this member"})
	}

	plan := models.WorkoutPlan{
		GymID:       gymID,
		TrainerID:   trainerID,
		MemberID:    req.MemberID,
		Title:       req.Title,
		Description: req.Description,
	}

	if err := database.DB.Create(&plan).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not create workout plan"})
	}

	return c.JSON(http.StatusCreated, plan)
}

// GetWorkoutPlans - A user can see workout plans according to their role.
func GetWorkoutPlans(c echo.Context) error {
	var plans []models.WorkoutPlan
	query := database.DB.Model(&models.WorkoutPlan{})

	roleRaw := c.Get("role")
	role, ok := roleRaw.(string)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	switch role {
	case "SuperAdmin":
		if gymID := c.QueryParam("gym_id"); gymID != "" {
			query = query.Where("gym_id = ?", gymID)
		}
	case "GymAdmin":
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Gym ID required"})
		}
		query = query.Where("gym_id = ?", uint(gymIDRaw.(float64)))
	case "Trainer":
		userIDRaw := c.Get("user_id")
		if userIDRaw == nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		}
		query = query.Where("trainer_id = ?", uint(userIDRaw.(float64)))
	default: // Member
		userIDRaw := c.Get("user_id")
		if userIDRaw == nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		}
		query = query.Where("member_id = ?", uint(userIDRaw.(float64)))
	}

	if targetMemberID := c.QueryParam("member_id"); targetMemberID != "" {
		query = query.Where("member_id = ?", targetMemberID)
	}
	if targetTrainerID := c.QueryParam("trainer_id"); targetTrainerID != "" {
		query = query.Where("trainer_id = ?", targetTrainerID)
	}

	if err := query.Order("created_at DESC").Find(&plans).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not fetch workout plans"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count": len(plans),
		"plans": plans,
	})
}

// UpdateWorkoutPlanRequest only exposes fields that are safe to modify.
// ID, GymID, TrainerID, MemberID, and timestamps are intentionally excluded
// and cannot be changed via the update endpoint.
// Pointer fields allow partial updates: omitted fields remain untouched.
type UpdateWorkoutPlanRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
}

// UpdateWorkoutPlan - The trainer who created it, GymAdmin, or SuperAdmin can update.
func UpdateWorkoutPlan(c echo.Context) error {
	id := c.Param("id")

	var plan models.WorkoutPlan
	if err := database.DB.First(&plan, id).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Workout plan not found"})
	}

	role := c.Get("role").(string)
	userIDRaw := c.Get("user_id")
	if userIDRaw == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}
	userID := uint(userIDRaw.(float64))

	// Trainers can only update plans they created
	if role == "Trainer" && plan.TrainerID != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only update workout plans you created"})
	}

	var req UpdateWorkoutPlanRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	// Build update map with only the fields that were actually provided.
	// Protected fields (ID, GymID, TrainerID, MemberID, timestamps) are never touched.
	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}

	if len(updates) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No fields provided to update"})
	}

	if err := database.DB.Model(&plan).Updates(updates).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not update workout plan"})
	}

	return c.JSON(http.StatusOK, plan)
}

// DeleteWorkoutPlan - The trainer who created it, GymAdmin, or SuperAdmin can delete.
func DeleteWorkoutPlan(c echo.Context) error {
	id := c.Param("id")

	var plan models.WorkoutPlan
	if err := database.DB.First(&plan, id).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Workout plan not found"})
	}

	role := c.Get("role").(string)
	userIDRaw := c.Get("user_id")
	if userIDRaw == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}
	userID := uint(userIDRaw.(float64))

	// Trainers can only delete plans they created
	if role == "Trainer" && plan.TrainerID != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only delete workout plans you created"})
	}

	if err := database.DB.Delete(&plan).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not delete workout plan"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Workout plan deleted successfully"})
}
