package handlers

import (
	"net/http"
	"strconv"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

type CreateMembershipPlanRequest struct {
	Name           string  `json:"name"`
	DurationMonths int     `json:"duration_months"`
	Price          float64 `json:"price"`
}

func CreateMembershipPlan(c echo.Context) error {
	var req CreateMembershipPlanRequest
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

	plan := models.MembershipPlan{
		GymID:          uint(gymIDFromParam),
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

func UpdateMembershipPlan(c echo.Context) error {
	planID := c.Param("membershipId")
	gymIDParam := c.Param("gymId")

	gymIDFromParam, err := strconv.ParseUint(gymIDParam, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid Gym ID"})
	}

	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	updateData := make(map[string]interface{})
	if val, ok := req["name"]; ok {
		updateData["name"] = val
	}
	if val, ok := req["duration_months"]; ok {
		updateData["duration_months"] = val
	}
	if val, ok := req["price"]; ok {
		updateData["price"] = val
	}
	if val, ok := req["is_active"]; ok {
		updateData["is_active"] = val
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
		return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to update this plan"})
	}

	if err := database.DB.Model(&plan).Updates(updateData).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not update plan"})
	}

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

	return c.JSON(http.StatusOK, map[string]interface{}{"count": len(plans), "plans": plans})
}

func GetMembershipPlans(c echo.Context) error {
	var plans []models.MembershipPlan
	
	gymIDRaw := c.Get("gym_id")
	if gymIDRaw != nil {	
		if err := database.DB.Where("gym_id = ?", uint(gymIDRaw.(float64))).Find(&plans).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not fetch plans"})
		}
	} else {
		// SuperAdmin might request all plans? Or maybe we require a gym_id query param
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
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"count": len(plans), "plans": plans})
}
