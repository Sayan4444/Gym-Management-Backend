package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

func GetUsers(c echo.Context) error {
	var users []models.User
	query := database.DB.Model(&models.User{})

	adminRoleRaw := c.Get("role")
	adminRole, _ := adminRoleRaw.(string)

	switch adminRole {
	case "SuperAdmin":
		// SuperAdmin can see all users, no base filter applied
		if c.QueryParam("gym_id") != "" {
			query = query.Where("gym_id = ?", c.QueryParam("gym_id"))
		}
	case "GymAdmin":
		// GymAdmin can only see users in their own gym
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw != nil {
			query = query.Where("gym_id = ?", uint(gymIDRaw.(float64)))
		} else {
			query = query.Where("id = 0") // fallback to block access if no gym_id
		}
	case "Trainer":
		// Trainer can only see users under them in their gym
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw != nil {
			query = query.Where("gym_id = ?", uint(gymIDRaw.(float64)))
		} else {
			query = query.Where("id = 0")
		}

		userIDRaw := c.Get("user_id")
		if userIDRaw != nil {
			query = query.Where("trainer_id = ?", uint(userIDRaw.(float64))).Where("role = ?", "Member")
		} else {
			query = query.Where("id = 0")
		}
	default:
		query = query.Where("id = 0") // Deny by default
	}

	// Add this block to filter by trainer_id from query params
    trainerIDFilter := c.QueryParam("trainer_id")
    if trainerIDFilter != "" {
        query = query.Where("users.trainer_id = ?", trainerIDFilter)
    }

	roleFilter := c.QueryParam("role")
	if roleFilter != "" {
		query = query.Where("users.role = ?", roleFilter)
	}

	if c.QueryParam("is_premium") == "true" {
		query = query.Joins("JOIN subscriptions ON subscriptions.user_id = users.id").
			Joins("JOIN membership_plans ON membership_plans.id = subscriptions.plan_id").
			Where("subscriptions.status = ?", "Active").
			Where("membership_plans.name ILIKE ?", "%premium%")
	} else if subStatus := c.QueryParam("subscription_status"); subStatus != "" && subStatus != "all" {
		query = query.Joins("LEFT JOIN subscriptions ON subscriptions.user_id = users.id")
		if subStatus == "none" {
			query = query.Where("subscriptions.id IS NULL")
		} else {
			query = query.Where("subscriptions.status = ?", subStatus)
		}
	}

	if search := c.QueryParam("search"); search != "" {
		query = query.Where("users.name ILIKE ? OR users.email ILIKE ? OR users.phone ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

    if includeParam := c.QueryParam("include"); includeParam != "" {
        includes := strings.SplitSeq(includeParam, ",")
        for relation := range includes {
            switch strings.ToLower(strings.TrimSpace(relation)) {
            case "gym":
                query = query.Preload("Gym")
            case "subscription":
                query = query.Preload("Subscription").Preload("Subscription.Plan")
            case "trainer":
                query = query.Preload("Trainer")
            case "workout_plan":
                query = query.Preload("WorkoutPlans")
            }
        }
    }

	query = query.Order("users.created_at DESC")

	if err := query.Find(&users).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch users"})
	}

	return c.JSON(http.StatusOK, map[string]any{"count": len(users), "users": users})
}

type UpdateProfileRequest struct {
	Name                  *string   `json:"name"`
	Phone                 *string   `json:"phone"`
	DOB                   *string   `json:"dob"`
	Gender                *string   `json:"gender"`
	Address               *string   `json:"address"`
	EmergencyContactName  *string   `json:"emergency_contact_name"`
	EmergencyContactPhone *string   `json:"emergency_contact_phone"`
	BloodGroup            *string   `json:"blood_group"`
	Height                *float64 `json:"height"`
	Weight                *float64 `json:"weight"`
	MedicalConditions     *string   `json:"medical_conditions"`
}

func UpdateProfile(c echo.Context) error {
	userIDRaw := c.Get("user_id")
	if userIDRaw == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	loggedInUserID := uint(userIDRaw.(float64))
	role := c.Get("role").(string)

	targetIDStr := c.Param("id")
	targetID, err := strconv.ParseUint(targetIDStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	var user models.User
	if err := database.DB.First(&user, uint(targetID)).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	// Permission Checks
	switch role {
	case "SuperAdmin":
		// Can update any user
	case "GymAdmin":
		// GymAdmin can only update users in their gym
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
		}
	default:
		// Trainer and Member can only update themselves
		if uint(targetID) != loggedInUserID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
		}
	}

	var req UpdateProfileRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	if err := database.DB.Model(&user).Updates(req).Error; err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not update profile"})
    }

	database.DB.First(&user, uint(targetID))
	return c.JSON(http.StatusOK, user)
}

func DeleteProfile(c echo.Context) error {
	targetIDStr := c.Param("id")
	targetID, err := strconv.ParseUint(targetIDStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	var user models.User
	if err := database.DB.First(&user, uint(targetID)).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Profile not found"})
	}

	adminRoleRaw := c.Get("role")
	adminRole, ok := adminRoleRaw.(string)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	userIDRaw := c.Get("user_id")
	if userIDRaw == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}
	loggedInUserID := uint(userIDRaw.(float64))

	// Permission Checks
	switch adminRole {
	case "SuperAdmin":
		if uint(targetID) == loggedInUserID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "SuperAdmin cannot delete their own profile"})
		}
		// Can delete any other user
	case "GymAdmin":
		if uint(targetID) == loggedInUserID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "GymAdmin cannot delete their own profile"})
		}
		// GymAdmin can only delete users in their gym
		adminGymIDRaw := c.Get("gym_id")
		if adminGymIDRaw == nil || user.GymID == nil || uint(adminGymIDRaw.(float64)) != *user.GymID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions to delete profile in another gym"})
		}
	default:
		// Trainer and Member can only delete themselves
		if uint(targetID) != loggedInUserID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
		}
	}

	if err := database.DB.Delete(&user).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not delete user"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Profile deleted successfully"})
}
