package handlers

import (
	"net/http"
	"strconv"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

type MembershipPlanRequest struct {
	Name           *string  `json:"name"`
	DurationMonths *int     `json:"duration_months"`
	Price          *float64 `json:"price"`
	IsActive       *bool   `json:"is_active"`
}

func CreateMembershipPlan(c echo.Context) error {
	var req MembershipPlanRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	gymIDParam := c.Param("gymId")
	gymIDFromParam, err := strconv.ParseUint(gymIDParam, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Gym ID"})
	}

	gymIDRaw := c.Get("gym_id")
	if gymIDRaw == nil || uint(gymIDRaw.(float64)) != uint(gymIDFromParam) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to create plans for this gym"})
	}

	if req.Name == nil || req.DurationMonths == nil || req.Price == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Missing required fields"})
	}

	plan := models.MembershipPlan{
		GymID:          uint(gymIDFromParam),
		Name:           *req.Name,
		DurationMonths: *req.DurationMonths,
		Price:          *req.Price,
		IsActive:       true,
	}

	if err := database.DB.Create(&plan).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not create plan"})
	}

	return c.JSON(http.StatusCreated, plan)
}

func UpdateMembershipPlan(c echo.Context) error {
	planID := c.Param("membershipId")
	gymIDParam := c.Param("gymId")

	gymIDFromParam, err := strconv.ParseUint(gymIDParam, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Gym ID"})
	}

	// 1. Fetch the existing plan first to check existence and ownership
	var plan models.MembershipPlan
	if err := database.DB.First(&plan, planID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Plan not found"})
	}

	// Verify the plan actually belongs to the gym specified in the URL
	if plan.GymID != uint(gymIDFromParam) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Plan does not belong to the specified gym"})
	}

	// 2. Permission Checks using switch
	role := c.Get("role").(string)

	switch role {
	case "SuperAdmin":
		// SuperAdmin can update plans for any gym
	case "GymAdmin":
		// GymAdmin can only update plans belonging to their specific gym
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil || uint(gymIDRaw.(float64)) != plan.GymID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to update this plan"})
		}
	default:
		// Trainers or Members cannot update plans
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
	}

	// 3. Bind the incoming JSON to our pointer-based request struct
	var req MembershipPlanRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	// 4. Perform the update. GORM will only update fields that are not nil in the req struct.
	if err := database.DB.Model(&plan).Updates(req).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not update plan"})
	}

	// Fetch the fully updated plan from the DB to return to the client
	database.DB.First(&plan, plan.ID)
	return c.JSON(http.StatusOK, plan)
}

func DeleteMembershipPlan(c echo.Context) error {
	planID := c.Param("membershipId")
	gymIDParam := c.Param("gymId")

	gymIDFromParam, err := strconv.ParseUint(gymIDParam, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Gym ID"})
	}

	var plan models.MembershipPlan
	if err := database.DB.First(&plan, planID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Plan not found"})
	}

	if plan.GymID != uint(gymIDFromParam) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Plan does not belong to the specified gym"})
	}

	gymIDRaw := c.Get("gym_id")
	if gymIDRaw == nil || uint(gymIDRaw.(float64)) != plan.GymID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to delete this plan"})
	}

	if err := database.DB.Delete(&plan).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not delete plan"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Plan deleted successfully"})
}

func GetMembershipPlansByGym(c echo.Context) error {
	gymID := c.Param("gymId")

	var plans []models.MembershipPlan
	if err := database.DB.Where("gym_id = ? AND is_active = ?", gymID, true).Find(&plans).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not retrieve plans"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"count": len(plans), "memberships": plans})
}

func GetMembershipPlans(c echo.Context) error {
	var plans []models.MembershipPlan
	role := c.Get("role").(string)

	switch role {
	case "SuperAdmin":
		// SuperAdmin can fetch all plans, or filter by a specific gym using ?gym_id=123
		gymIDStr := c.QueryParam("gym_id")
		
		if gymIDStr != "" {
			if err := database.DB.Where("gym_id = ?", gymIDStr).Find(&plans).Error; err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not fetch plans"})
			}
		} else {
			if err := database.DB.Find(&plans).Error; err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not fetch plans"})
			}
		}

	case "GymAdmin", "Trainer", "Member":
		// Standard roles can only view membership plans associated with their own gym
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied. No gym associated with your account."})
		}
		
		gymID := uint(gymIDRaw.(float64))
		if err := database.DB.Where("gym_id = ?", gymID).Find(&plans).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not fetch plans"})
		}

	default:
		// Catch-all for any unknown roles
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":       len(plans),
		"memberships": plans,
	})
}