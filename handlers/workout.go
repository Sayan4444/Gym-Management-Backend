package handlers

import (
	"net/http"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// ExerciseInput is a single exercise row sent from the client.
type ExerciseInput struct {
	Name string `json:"name"`
}

type CreateWorkoutPlanRequest struct {
	MemberID  uint            `json:"member_id"`
	Title     string          `json:"title"`
	Exercises []ExerciseInput `json:"exercises"`
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

	// Build exercises slice
	exercises := make([]models.WorkoutExercise, 0, len(req.Exercises))
	for _, ex := range req.Exercises {
		exercises = append(exercises, models.WorkoutExercise{Name: ex.Name})
	}

	plan := models.WorkoutPlan{
		GymID:     gymID,
		TrainerID: trainerID,
		MemberID:  req.MemberID,
		Title:     req.Title,
		Exercises: exercises,
	}

	if err := database.DB.Create(&plan).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not create workout plan"})
	}

	return c.JSON(http.StatusCreated, plan)
}

// GetWorkoutPlans - A user can see workout plans according to their role.
func GetWorkoutPlans(c echo.Context) error {
	var plans []models.WorkoutPlan
	query := database.DB.Model(&models.WorkoutPlan{}).Preload("Exercises")

	roleRaw := c.Get("role")
	role, ok := roleRaw.(string)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	switch role {
	case "SuperAdmin":
		// SuperAdmins can see everything, but can optionally scope down to a specific gym via query params
		if gymID := c.QueryParam("gym_id"); gymID != "" {
			query = query.Where("gym_id = ?", gymID)
		}
	case "GymAdmin":
		// GymAdmins are strictly restricted to data within their own gym
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Gym ID required"})
		}
		query = query.Where("gym_id = ?", uint(gymIDRaw.(float64)))
	case "Trainer":
		// Trainers can only see workout plans they have authored
		userIDRaw := c.Get("user_id")
		if userIDRaw == nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		}
		query = query.Where("trainer_id = ?", uint(userIDRaw.(float64)))
	default: // Member
		// Members can only see workout plans assigned to them
		userIDRaw := c.Get("user_id")
		if userIDRaw == nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		}
		query = query.Where("member_id = ?", uint(userIDRaw.(float64)))
	}

	// Apply additional optional URL query parameters (e.g., ?member_id=5&trainer_id=2)
	// Note: These append to the role-based restrictions above, they do not bypass them.
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

type UpdateWorkoutPlanRequest struct {
	Title     *string         `json:"title"`
	Exercises []ExerciseInput `json:"exercises"`
}

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

	// Handle role-based authorization using a switch case
	switch role {
	case "Trainer":
		// Trainers can only update plans they created
		if plan.TrainerID != userID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only update workout plans you created"})
		}
	case "GymAdmin":
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Gym ID required"})
		}
		if plan.GymID != uint(gymIDRaw.(float64)) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only update workout plans within your own gym"})
		}
	default:
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to update workout plans"})
	}

	var req UpdateWorkoutPlanRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	// Run update + exercise replacement in a transaction
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Update title if provided
		if req.Title != nil {
			if err := tx.Model(&plan).Update("title", *req.Title).Error; err != nil {
				return err
			}
		}

		// Delete all existing exercises for this plan, then insert new ones
		if err := tx.Where("workout_plan_id = ?", plan.ID).Delete(&models.WorkoutExercise{}).Error; err != nil {
			return err
		}
		for _, ex := range req.Exercises {
			exercise := models.WorkoutExercise{
				WorkoutPlanID: plan.ID,
				Name:          ex.Name,
			}
			if err := tx.Create(&exercise).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not update workout plan"})
	}

	// Reload plan with exercises
	if err := database.DB.Preload("Exercises").First(&plan, plan.ID).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not reload workout plan"})
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

	// Handle role-based authorization using a switch case
	switch role {
	case "Trainer":
		if plan.TrainerID != userID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only delete workout plans you created"})
		}
	case "GymAdmin":
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Gym ID required"})
		}
		if plan.GymID != uint(gymIDRaw.(float64)) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only delete workout plans within your own gym"})
		}
	case "SuperAdmin":
		// SuperAdmin can delete any plan
	default:
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to delete workout plans"})
	}

	// Delete child exercises first (soft-delete), then the plan
	database.DB.Where("workout_plan_id = ?", plan.ID).Delete(&models.WorkoutExercise{})

	if err := database.DB.Delete(&plan).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not delete workout plan"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Workout plan deleted successfully"})
}
