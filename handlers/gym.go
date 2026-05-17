package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"gym-saas/database"
	"gym-saas/models"
	"gym-saas/utils"

	"github.com/labstack/echo/v4"
)

func GetDashboardStats(c echo.Context) error {
	gymIDRaw := c.Get("gym_id")
	role := c.Get("role").(string)

	var gymID uint
	hasGymID := false
	if gymIDRaw != nil {
		gymID = uint(gymIDRaw.(float64))
		hasGymID = true
	} else if role != "SuperAdmin" {
		log.Printf("API Error (http.StatusForbidden): Missing gym_id")
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Missing gym_id"})
	}

	var totalMembers int64
	qMem := database.DB.Model(&models.User{}).Where("role = ?", "Member")
	if hasGymID {
		qMem = qMem.Where("gym_id = ?", gymID)
	}
	qMem.Count(&totalMembers)

	var activeMemberships int64
	qSub := database.DB.Model(&models.Subscription{}).
		Joins("JOIN users ON users.id = subscriptions.user_id").
		Where("subscriptions.status = ?", "Active")
	if hasGymID {
		qSub = qSub.Where("users.gym_id = ?", gymID)
	}
	qSub.Count(&activeMemberships)

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var todaysAttendance int64
	qAtt := database.DB.Model(&models.Attendance{}).
		Joins("JOIN users ON users.id = attendances.user_id").
		Where("attendances.date = ?", today)
	if hasGymID {
		qAtt = qAtt.Where("users.gym_id = ?", gymID)
	}
	qAtt.Count(&todaysAttendance)

	// Revenue (sum of all paid payments)
	type Result struct {
		Total float64
	}
	var res Result
	qPay := database.DB.Model(&models.Payment{}).
		Select("sum(payments.amount) as total").
		Joins("JOIN subscriptions ON subscriptions.id = payments.subscription_id").
		Joins("JOIN users ON users.id = subscriptions.user_id").
		Where("payments.status = ?", "Paid")
	if hasGymID {
		qPay = qPay.Where("users.gym_id = ?", gymID)
	}
	qPay.Scan(&res)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"total_members":      totalMembers,
		"active_memberships": activeMemberships,
		"todays_attendance":  todaysAttendance,
		"total_revenue":      res.Total,
	})
}

func GetGyms(c echo.Context) error {
	var gyms []models.Gym
	if err := database.DB.Find(&gyms).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch gyms"})
	}
	return c.JSON(http.StatusOK, map[string]any{"count": len(gyms), "gyms": gyms})
}

func GetGymIDFromDomain(c echo.Context) error {
	domainName := c.Param("domainName")
	var gym models.Gym

	if err := database.DB.Select("id").Where("domain = ?", domainName).First(&gym).Error; err != nil {
		log.Printf("Error fetching gym ID by domain: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Gym not found for this domain"})
	}

	return c.JSON(http.StatusOK, map[string]uint{"id": gym.ID})
}

func GetGym(c echo.Context) error {
	gymID := c.Param("id")
	var gym models.Gym
	// Build base query with any requested preloads
	query := database.DB.Model(&models.Gym{})
	if includeParam := c.QueryParam("include"); includeParam != "" {
		includes := strings.Split(includeParam, ",")
		for _, relation := range includes {
			switch strings.ToLower(strings.TrimSpace(relation)) {
			case "users":
				query = query.Preload("Users")
			case "membership_plans":
				query = query.Preload("MembershipPlans.PlanAddons.Addon")
			case "addons":
				query = query.Preload("Addons")
			}
		}
	}

	if err := query.First(&gym, gymID).Error; err != nil {
		log.Printf("Error fetching by ID: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Gym not found"})
	}

	return c.JSON(http.StatusOK, gym)
}

type GymRequest struct {
	Name     *string `json:"name" form:"name"`
	Slug     *string `json:"slug" form:"slug"`
	Domain   *string `json:"domain" form:"domain"`
	Address  *string `json:"address" form:"address"`
	Whatsapp *string `json:"whatsapp" form:"whatsapp"`
	Email    *string `json:"email" form:"email"`
	Phone    *string `json:"phone" form:"phone"`
}

func AddGym(c echo.Context) error {
	var req GymRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := database.DB.Create(&req).Error; err != nil {

		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, req)
}

func UpdateGym(c echo.Context) error {
	gymID := c.Param("id")
	role := c.Get("role").(string)

	var gym models.Gym

	if err := database.DB.First(&gym, gymID).Error; err != nil {
		log.Printf("Error fetching gym: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Gym not found"})
	}

	// Permission Checks using switch
	switch role {
	case "SuperAdmin":
		// SuperAdmin can update any gym
	case "GymAdmin":
		// GymAdmin can only update their own gym
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil {
			log.Printf("API Error (http.StatusForbidden): Access denied. No gym associated.")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied. No gym associated."})
		}
		userGymID := uint(gymIDRaw.(float64))
		if userGymID != gym.ID {
			log.Printf("API Error (http.StatusForbidden): Access denied. You can only update your own gym.")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied. You can only update your own gym."})
		}
	default:
		// Other roles (e.g., Trainer, Member) cannot update gym details
		log.Printf("API Error (http.StatusForbidden): Insufficient permissions")
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
	}

	var req GymRequest
	var raw map[string]interface{}
	isMultipart := strings.HasPrefix(c.Request().Header.Get("Content-Type"), "multipart/form-data")

	if isMultipart {
		file, err := c.FormFile("gym_icon")
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
			filename := fmt.Sprintf("gyms/%d/gym_icon%s", gym.ID, ext)

			url, err := utils.UploadToSpaces(src, filename, file.Header.Get("Content-Type"))
			if err != nil {
				log.Printf("Error from UploadToSpaces: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to upload image"})
			}
			gym.GymIcon = url
		}

		if err := c.Bind(&req); err != nil {
			log.Printf("Error: %v", err)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
		}

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
		if gym.GymIcon != "" {
			err := utils.DeleteFromSpaces(gym.GymIcon)
			if err != nil {
				log.Printf("Failed to delete old image from spaces: %v", err)
			}
		}
		gym.GymIcon = ""
	}

	if _, ok := raw["name"]; ok {
		if req.Name != nil {
			gym.Name = *req.Name
		} else {
			gym.Name = ""
		}
	}
	if _, ok := raw["slug"]; ok {
		if req.Slug != nil {
			gym.Slug = *req.Slug
		} else {
			gym.Slug = ""
		}
	}
	if _, ok := raw["domain"]; ok {
		if req.Domain != nil {
			gym.Domain = *req.Domain
		} else {
			gym.Domain = ""
		}
	}
	if _, ok := raw["address"]; ok {
		if req.Address != nil {
			gym.Address = *req.Address
		} else {
			gym.Address = ""
		}
	}
	if _, ok := raw["whatsapp"]; ok {
		if req.Whatsapp != nil {
			gym.Whatsapp = *req.Whatsapp
		} else {
			gym.Whatsapp = ""
		}
	}
	if _, ok := raw["email"]; ok {
		if req.Email != nil {
			gym.Email = *req.Email
		} else {
			gym.Email = ""
		}
	}
	if _, ok := raw["phone"]; ok {
		if req.Phone != nil {
			gym.Phone = *req.Phone
		} else {
			gym.Phone = ""
		}
	}

	if err := database.DB.Save(&gym).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update gym"})
	}

	// Fetch the updated gym to return the complete object
	database.DB.First(&gym, gym.ID)
	return c.JSON(http.StatusOK, gym)
}

func DeleteGym(c echo.Context) error {
	gymID := c.Param("id")

	var gym models.Gym

	if err := database.DB.First(&gym, gymID).Error; err != nil {
		log.Printf("Error fetching gym: %v", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Gym not found"})
	}

	if err := database.DB.Delete(&gym).Error; err != nil {

		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete gym"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Gym deleted successfully"})
}
