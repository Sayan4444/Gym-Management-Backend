package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"gym-saas/database"
	"gym-saas/models"
	"gym-saas/utils"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
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

		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch users"})
	}

	return c.JSON(http.StatusOK, map[string]any{"count": len(users), "users": users})
}

type UpdateProfileRequest struct {
	Name                  *string         `json:"name" form:"name"`
	Phone                 *string         `json:"phone" form:"phone"`
	DOB                   *string         `json:"dob" form:"dob"`
	Gender                *string         `json:"gender" form:"gender"`
	Address               *string         `json:"address" form:"address"`
	EmergencyContactName  *string         `json:"emergency_contact_name" form:"emergency_contact_name"`
	EmergencyContactPhone *string         `json:"emergency_contact_phone" form:"emergency_contact_phone"`
	BloodGroup            *string         `json:"blood_group" form:"blood_group"`
	Height                *float64        `json:"height" form:"height"`
	Weight                *float64        `json:"weight" form:"weight"`
	MedicalConditions     *string         `json:"medical_conditions" form:"medical_conditions"`
	Role                  *string         `json:"role" form:"role"`
	SocialMedia           *pq.StringArray `json:"social_media" form:"social_media"`
}

func UpdateProfile(c echo.Context) error {
	userIDRaw := c.Get("user_id")
	if userIDRaw == nil {
		log.Printf("API Error (http.StatusUnauthorized): Unauthorized")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	loggedInUserID := uint(userIDRaw.(float64))
	role := c.Get("role").(string)

	targetIDStr := c.Param("id")
	targetID, err := strconv.ParseUint(targetIDStr, 10, 32)
	if err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	var user models.User
	if err := database.DB.First(&user, uint(targetID)).Error; err != nil {
		log.Printf("Error: %v", err)
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
			log.Printf("API Error (http.StatusForbidden): Insufficient permissions")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
		}
	default:
		// Trainer and Member can only update themselves
		if uint(targetID) != loggedInUserID {
			log.Printf("API Error (http.StatusForbidden): Insufficient permissions")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
		}
	}

	var raw map[string]interface{}
	var req UpdateProfileRequest
	isMultipart := strings.HasPrefix(c.Request().Header.Get("Content-Type"), "multipart/form-data")

	if isMultipart {
		file, err := c.FormFile("image")
		if err == nil {
			src, err := file.Open()
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to process image"})
			}
			defer src.Close()

			ext := filepath.Ext(file.Filename)
			if ext == "" {
				ext = ".png"
			}
			filename := fmt.Sprintf("users/%d/profile_pic%s", targetID, ext)

			url, err := utils.UploadToSpaces(src, filename, file.Header.Get("Content-Type"))
			if err != nil {
				log.Printf("Error from UploadToSpaces: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to upload image"})
			}
			user.PhotoURL = url
		}

		if err := c.Bind(&req); err != nil {
			log.Printf("Error: %v", err)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
		}

		// Convert form data to raw map format so existing logic works
		raw = make(map[string]interface{})
		form, err := c.MultipartForm()
		if err == nil && form != nil {
			for key := range form.Value {
				raw[key] = true
			}
		}

	} else {
		bodyBytes, err := io.ReadAll(c.Request().Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}
		// Restore the body for Bind
		c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		if err := json.Unmarshal(bodyBytes, &raw); err != nil {
			log.Printf("Error unmarshaling to map: %v", err)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		}

		if err := c.Bind(&req); err != nil {
			log.Printf("Error: %v", err)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
		}
	}

	shouldRemove := false
	if isMultipart {
		if c.FormValue("remove_image") == "true" {
			shouldRemove = true
		}
	} else {
		if val, ok := raw["remove_image"]; ok {
			if b, isBool := val.(bool); isBool && b {
				shouldRemove = true
			} else if s, isStr := val.(string); isStr && s == "true" {
				shouldRemove = true
			}
		}
	}

	if shouldRemove {
		if user.PhotoURL != "" {
			err := utils.DeleteFromSpaces(user.PhotoURL)
			if err != nil {
				log.Printf("Failed to delete old image from spaces: %v", err)
			}
		}
		user.PhotoURL = ""
	}

	if req.Role != nil && *req.Role != user.Role {
		switch role {
		case "SuperAdmin":
			if *req.Role == "SuperAdmin" {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "Cannot grant SuperAdmin role"})
			}
		case "GymAdmin":
			if *req.Role == "SuperAdmin" || *req.Role == "GymAdmin" {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "GymAdmin cannot grant SuperAdmin or GymAdmin role"})
			}
		default:
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You do not have permission to change roles"})
		}
	}

	if _, ok := raw["name"]; ok {
		if req.Name != nil {
			user.Name = *req.Name
		} else {
			user.Name = ""
		}
	}
	if _, ok := raw["phone"]; ok {
		if req.Phone != nil {
			user.Phone = *req.Phone
		} else {
			user.Phone = ""
		}
	}
	if _, ok := raw["social_media"]; ok {
		if req.SocialMedia != nil {
			user.SocialMedia = *req.SocialMedia
		} else {
			user.SocialMedia = pq.StringArray{}
		}
	}
	if _, ok := raw["role"]; ok {
		if req.Role != nil {
			user.Role = *req.Role
		}
	}

	if _, ok := raw["dob"]; ok {
		if req.DOB != nil {
			user.DOB = *req.DOB
		} else {
			user.DOB = ""
		}
	}
	if _, ok := raw["gender"]; ok {
		if req.Gender != nil {
			user.Gender = *req.Gender
		} else {
			user.Gender = ""
		}
	}
	if _, ok := raw["address"]; ok {
		if req.Address != nil {
			user.Address = *req.Address
		} else {
			user.Address = ""
		}
	}
	if _, ok := raw["emergency_contact_name"]; ok {
		if req.EmergencyContactName != nil {
			user.EmergencyContactName = *req.EmergencyContactName
		} else {
			user.EmergencyContactName = ""
		}
	}
	if _, ok := raw["emergency_contact_phone"]; ok {
		if req.EmergencyContactPhone != nil {
			user.EmergencyContactPhone = *req.EmergencyContactPhone
		} else {
			user.EmergencyContactPhone = ""
		}
	}
	if _, ok := raw["blood_group"]; ok {
		if req.BloodGroup != nil {
			user.BloodGroup = *req.BloodGroup
		} else {
			user.BloodGroup = ""
		}
	}
	if _, ok := raw["height"]; ok {
		user.Height = req.Height
	}
	if _, ok := raw["weight"]; ok {
		user.Weight = req.Weight
	}
	if _, ok := raw["medical_conditions"]; ok {
		if req.MedicalConditions != nil {
			user.MedicalConditions = *req.MedicalConditions
		} else {
			user.MedicalConditions = ""
		}
	}

	if err := database.DB.Save(&user).Error; err != nil {

		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not update profile"})
	}

	database.DB.First(&user, uint(targetID))
	return c.JSON(http.StatusOK, user)
}

func DeleteProfile(c echo.Context) error {
	targetIDStr := c.Param("id")
	targetID, err := strconv.ParseUint(targetIDStr, 10, 32)
	if err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	var user models.User
	if err := database.DB.First(&user, uint(targetID)).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Profile not found"})
	}

	adminRoleRaw := c.Get("role")
	adminRole, ok := adminRoleRaw.(string)
	if !ok {
		log.Printf("API Error (http.StatusUnauthorized): Unauthorized")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	userIDRaw := c.Get("user_id")
	if userIDRaw == nil {
		log.Printf("API Error (http.StatusUnauthorized): Unauthorized")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}
	loggedInUserID := uint(userIDRaw.(float64))

	// Permission Checks
	switch adminRole {
	case "SuperAdmin":
		if uint(targetID) == loggedInUserID {
			log.Printf("API Error (http.StatusForbidden): SuperAdmin cannot delete their own profile")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "SuperAdmin cannot delete their own profile"})
		}
		// Can delete any other user
	case "GymAdmin":
		if uint(targetID) == loggedInUserID {
			log.Printf("API Error (http.StatusForbidden): GymAdmin cannot delete their own profile")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "GymAdmin cannot delete their own profile"})
		}
		// GymAdmin can only delete users in their gym
		adminGymIDRaw := c.Get("gym_id")
		if adminGymIDRaw == nil || user.GymID == nil || uint(adminGymIDRaw.(float64)) != *user.GymID {
			log.Printf("API Error (http.StatusForbidden): Insufficient permissions to delete profile in another gym")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions to delete profile in another gym"})
		}
	default:
		// Trainer and Member can only delete themselves
		if uint(targetID) != loggedInUserID {
			log.Printf("API Error (http.StatusForbidden): Insufficient permissions")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
		}
	}

	if err := database.DB.Delete(&user).Error; err != nil {

		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Could not delete user"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Profile deleted successfully"})
}
