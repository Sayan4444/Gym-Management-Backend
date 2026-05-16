package handlers

import (
	"log"
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
	IsActive       *bool    `json:"is_active"`
}

func CreateMembershipPlan(c echo.Context) error {
	var req MembershipPlanRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	gymIDParam := c.Param("gymId")
	gymIDFromParam, err := strconv.ParseUint(gymIDParam, 10, 32)
	if err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Gym ID"})
	}

	gymIDRaw := c.Get("gym_id")
	if gymIDRaw == nil || uint(gymIDRaw.(float64)) != uint(gymIDFromParam) {
		log.Printf("API Error (http.StatusForbidden): You do not have permission to create plans for this gym")
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to create plans for this gym"})
	}

	if req.Name == nil || req.DurationMonths == nil || req.Price == nil {
		log.Printf("API Error (http.StatusBadRequest): Missing required fields")
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

		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not create plan"})
	}

	// Preload PlanAddons for the response
	database.DB.Preload("PlanAddons.Addon").First(&plan, plan.ID)
	return c.JSON(http.StatusCreated, plan)
}

func UpdateMembershipPlan(c echo.Context) error {
	planID := c.Param("membershipId")
	gymIDParam := c.Param("gymId")

	gymIDFromParam, err := strconv.ParseUint(gymIDParam, 10, 32)
	if err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Gym ID"})
	}

	// 1. Fetch the existing plan first to check existence and ownership
	var plan models.MembershipPlan
	if err := database.DB.First(&plan, planID).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Plan not found"})
	}

	// Verify the plan actually belongs to the gym specified in the URL
	if plan.GymID != uint(gymIDFromParam) {
		log.Printf("API Error (http.StatusForbidden): Plan does not belong to the specified gym")
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
			log.Printf("API Error (http.StatusForbidden): You do not have permission to update this plan")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to update this plan"})
		}
	default:
		// Trainers or Members cannot update plans
		log.Printf("API Error (http.StatusForbidden): Insufficient permissions")
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
	}

	// 3. Bind the incoming JSON to our pointer-based request struct
	var req MembershipPlanRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	// 4. Perform the update. GORM will only update fields that are not nil in the req struct.
	if err := database.DB.Model(&plan).Updates(req).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not update plan"})
	}

	// Fetch the fully updated plan with PlanAddons from the DB to return to the client
	database.DB.Preload("PlanAddons.Addon").First(&plan, plan.ID)
	return c.JSON(http.StatusOK, plan)
}

func DeleteMembershipPlan(c echo.Context) error {
	planID := c.Param("membershipId")
	gymIDParam := c.Param("gymId")

	gymIDFromParam, err := strconv.ParseUint(gymIDParam, 10, 32)
	if err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Gym ID"})
	}

	var plan models.MembershipPlan
	if err := database.DB.First(&plan, planID).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Plan not found"})
	}

	if plan.GymID != uint(gymIDFromParam) {
		log.Printf("API Error (http.StatusForbidden): Plan does not belong to the specified gym")
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Plan does not belong to the specified gym"})
	}

	gymIDRaw := c.Get("gym_id")
	if gymIDRaw == nil || uint(gymIDRaw.(float64)) != plan.GymID {
		log.Printf("API Error (http.StatusForbidden): You do not have permission to delete this plan")
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to delete this plan"})
	}

	if err := database.DB.Delete(&plan).Error; err != nil {

		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not delete plan"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Plan deleted successfully"})
}

func GetMembershipPlansByGym(c echo.Context) error {
	gymID := c.Param("gymId")

	var plans []models.MembershipPlan
	if err := database.DB.Preload("PlanAddons.Addon").Where("gym_id = ? AND is_active = ?", gymID, true).Find(&plans).Error; err != nil {
		log.Printf("Error: %v", err)
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
			if err := database.DB.Preload("PlanAddons.Addon").Where("gym_id = ?", gymIDStr).Find(&plans).Error; err != nil {
				log.Printf("Error: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not fetch plans"})
			}
		} else {
			if err := database.DB.Preload("PlanAddons.Addon").Find(&plans).Error; err != nil {
				log.Printf("Error: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not fetch plans"})
			}
		}

	case "GymAdmin", "Trainer", "Member":
		// Standard roles can only view membership plans associated with their own gym
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil {
			log.Printf("API Error (http.StatusForbidden): Access denied. No gym associated with your account.")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied. No gym associated with your account."})
		}

		gymID := uint(gymIDRaw.(float64))
		if err := database.DB.Preload("PlanAddons.Addon").Where("gym_id = ? AND is_active = ?", gymID, true).Find(&plans).Error; err != nil {
			log.Printf("Error: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not fetch plans"})
		}

	default:
		// Catch-all for any unknown roles
		log.Printf("API Error (http.StatusForbidden): Insufficient permissions")
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":       len(plans),
		"memberships": plans,
	})
}

// ---------- Plan Addon Management Handlers ----------

type PlanAddonRequest struct {
	AddonID   *uint `json:"addon_id"`
	Frequency *int  `json:"frequency"` // total count of addon usage included in the plan (must be > 0)
}

// AddPlanAddon attaches an addon (with frequency) to a membership plan.
func AddPlanAddon(c echo.Context) error {
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

	var req PlanAddonRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}
	if req.AddonID == nil || req.Frequency == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "addon_id and frequency are required"})
	}
	if *req.Frequency <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "frequency must be a positive number"})
	}

	// Verify the addon belongs to the same gym
	var addon models.Addon
	if err := database.DB.First(&addon, *req.AddonID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Addon not found"})
	}
	if addon.GymID != uint(gymIDFromParam) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Addon does not belong to this gym"})
	}

	// Avoid duplicates
	var existing models.PlanAddon
	if err := database.DB.Where("plan_id = ? AND addon_id = ?", plan.ID, *req.AddonID).First(&existing).Error; err == nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "Addon already added to this plan"})
	}

	planAddon := models.PlanAddon{
		PlanID:    plan.ID,
		AddonID:   *req.AddonID,
		Frequency: *req.Frequency,
	}
	if err := database.DB.Create(&planAddon).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not add addon to plan"})
	}

	database.DB.Preload("Addon").First(&planAddon, planAddon.ID)
	return c.JSON(http.StatusCreated, planAddon)
}

// UpdatePlanAddon updates the frequency of an existing plan-addon link.
func UpdatePlanAddon(c echo.Context) error {
	planID := c.Param("membershipId")
	planAddonID := c.Param("planAddonId")
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

	var planAddon models.PlanAddon
	if err := database.DB.Where("id = ? AND plan_id = ?", planAddonID, plan.ID).First(&planAddon).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Plan addon not found"})
	}

	var req struct {
		Frequency *int `json:"frequency"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}
	if req.Frequency == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "frequency is required"})
	}
	if *req.Frequency <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "frequency must be a positive number"})
	}

	planAddon.Frequency = *req.Frequency
	if err := database.DB.Save(&planAddon).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not update plan addon"})
	}

	database.DB.Preload("Addon").First(&planAddon, planAddon.ID)
	return c.JSON(http.StatusOK, planAddon)
}

// RemovePlanAddon removes an addon from a membership plan.
func RemovePlanAddon(c echo.Context) error {
	planID := c.Param("membershipId")
	planAddonID := c.Param("planAddonId")
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

	var planAddon models.PlanAddon
	if err := database.DB.Where("id = ? AND plan_id = ?", planAddonID, plan.ID).First(&planAddon).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Plan addon not found"})
	}

	if err := database.DB.Delete(&planAddon).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not remove addon from plan"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Addon removed from plan"})
}
