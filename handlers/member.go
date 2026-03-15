package handlers

import (
	"net/http"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

type CreateMemberRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
	PlanID   uint   `json:"plan_id"`
}

type EditMemberRequest struct {
	Name                  string   `json:"name"`
	Email                 string   `json:"email"`
	Phone                 string   `json:"phone"`
	DOB                   string   `json:"dob"`
	Gender                string   `json:"gender"`
	PhotoURL              string   `json:"photo_url"`
	Address               string   `json:"address"`
	EmergencyContactName  string   `json:"emergency_contact_name"`
	EmergencyContactPhone string   `json:"emergency_contact_phone"`
	BloodGroup            string   `json:"blood_group"`
	Height                *float64 `json:"height"`
	Weight                *float64 `json:"weight"`
	MedicalConditions     string   `json:"medical_conditions"`
}

func GetMembers(c echo.Context) error {
	var members []models.User
	query := database.DB.Where("role = ?", "Member")

	gymIDRaw := c.Get("gym_id")
	if gymIDRaw != nil {
		query = query.Where("gym_id = ?", uint(gymIDRaw.(float64)))
	} else if c.QueryParam("gym_id") != "" {
		query = query.Where("gym_id = ?", c.QueryParam("gym_id"))
	}

	if c.QueryParam("is_premium") == "true" {
		query = query.Joins("JOIN subscriptions ON subscriptions.user_id = users.id").
			Joins("JOIN membership_plans ON membership_plans.id = subscriptions.plan_id").
			Where("subscriptions.status = ?", "Active").
			Where("membership_plans.name ILIKE ?", "%premium%")
	}

	if err := query.Find(&members).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch members"})
	}

	return c.JSON(http.StatusOK, members)
}

func EditMember(c echo.Context) error {
	memberID := c.Param("id")
	var req EditMemberRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	var user models.User
	if err := database.DB.First(&user, memberID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Member not found"})
	}

	if user.Role != "Member" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "User is not a member"})
	}

	adminRoleRaw := c.Get("role")
	adminRole, ok := adminRoleRaw.(string)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	if adminRole == "GymAdmin" {
		adminGymIDRaw := c.Get("gym_id")
		if adminGymIDRaw == nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
		}
		adminGymID := uint(adminGymIDRaw.(float64))
		if user.GymID == nil || *user.GymID != adminGymID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions to edit member in another gym"})
		}
	} else if adminRole != "SuperAdmin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.DOB != "" {
		user.DOB = req.DOB
	}
	if req.Gender != "" {
		user.Gender = req.Gender
	}
	if req.PhotoURL != "" {
		user.PhotoURL = req.PhotoURL
	}
	if req.Address != "" {
		user.Address = req.Address
	}
	if req.EmergencyContactName != "" {
		user.EmergencyContactName = req.EmergencyContactName
	}
	if req.EmergencyContactPhone != "" {
		user.EmergencyContactPhone = req.EmergencyContactPhone
	}
	if req.BloodGroup != "" {
		user.BloodGroup = req.BloodGroup
	}
	if req.Height != nil {
		user.Height = req.Height
	}
	if req.Weight != nil {
		user.Weight = req.Weight
	}
	if req.MedicalConditions != "" {
		user.MedicalConditions = req.MedicalConditions
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not update user"})
	}

	return c.JSON(http.StatusOK, user)
}

func DeleteMember(c echo.Context) error {
	memberID := c.Param("id")

	var user models.User
	if err := database.DB.First(&user, memberID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Member not found"})
	}

	if user.Role != "Member" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "User is not a member"})
	}

	adminRoleRaw := c.Get("role")
	adminRole, ok := adminRoleRaw.(string)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	if adminRole == "GymAdmin" {
		adminGymIDRaw := c.Get("gym_id")
		if adminGymIDRaw == nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
		}
		adminGymID := uint(adminGymIDRaw.(float64))
		if user.GymID == nil || *user.GymID != adminGymID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions to delete member in another gym"})
		}
	} else if adminRole != "SuperAdmin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
	}

	if err := database.DB.Delete(&user).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not delete user"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Member deleted successfully"})
}
