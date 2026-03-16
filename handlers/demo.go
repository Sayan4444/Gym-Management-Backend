package handlers

import (
	"fmt"
	"log"
	"net/http"

	"gym-saas/database"
	"gym-saas/models"
	"gym-saas/utils"

	"github.com/labstack/echo/v4"
)

type DemoRequest struct {
	FullName      string `json:"fullName"`
	Mobile        string `json:"mobile"`
	Email         string `json:"email"`
	PreferredDate string `json:"preferredDate"`
	PreferredTime string `json:"preferredTime"`
	Notes         string `json:"notes"`
}

func SubmitDemoRequest(c echo.Context) error {
	var req DemoRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	if req.FullName == "" || req.Mobile == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Full name and mobile are required"})
	}

	// Find all super admins
	var superAdmins []models.User
	if err := database.DB.Where("role = ?", "SuperAdmin").Find(&superAdmins).Error; err != nil {
		log.Printf("Failed to query super admins: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
	}

	// Build email content
	subject := "New Demo Request from " + req.FullName

	emailLine := "Not provided"
	if req.Email != "" {
		emailLine = req.Email
	}
	dateLine := "Not specified"
	if req.PreferredDate != "" {
		dateLine = req.PreferredDate
	}
	timeLine := "Not specified"
	if req.PreferredTime != "" {
		timeLine = req.PreferredTime
	}
	notesLine := "None"
	if req.Notes != "" {
		notesLine = req.Notes
	}

	body := fmt.Sprintf(
		"Hi,\n\nA new demo request has been submitted.\n\nName: %s\nMobile: %s\nEmail: %s\nPreferred Date: %s\nPreferred Time: %s\nNotes: %s\n\nPlease follow up at the earliest.\n\n— GymFlow System",
		req.FullName, req.Mobile, emailLine, dateLine, timeLine, notesLine,
	)

	// Send to each super admin
	sentCount := 0
	for _, admin := range superAdmins {
		if admin.Email == "" {
			continue
		}
		if err := utils.SendEmail(admin.Email, subject, body); err != nil {
			log.Printf("Could not email %s: %v", admin.Email, err)
			// Log the full email for dev visibility when SMTP isn't configured
			log.Printf("Email content for %s:\nSubject: %s\n%s", admin.Email, subject, body)
		} else {
			sentCount++
		}
	}

	log.Printf("Demo request from %s (%s) — notified %d/%d super admins", req.FullName, req.Mobile, sentCount, len(superAdmins))

	return c.JSON(http.StatusOK, map[string]string{"message": "Demo request submitted successfully"})
}
