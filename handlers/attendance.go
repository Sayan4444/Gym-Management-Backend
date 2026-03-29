package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)

// ---------------------------------------------------------------------------
// Helper – generate a cryptographically secure random hex token
// ---------------------------------------------------------------------------

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ---------------------------------------------------------------------------
// rotateToken creates (or updates) the active QR token for the gym.
// ---------------------------------------------------------------------------

func rotateToken(gymID uint) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", err
	}

	// Token is valid for 30 seconds.
	expiresAt := time.Now().UTC().Add(30 * time.Second)

	// Upsert: one row per gym. 
	// Assign ensures that if the record exists, it updates the Token and ExpiresAt fields.
	result := database.DB.Where(models.GymQRToken{GymID: gymID}).
		Assign(models.GymQRToken{Token: token, ExpiresAt: expiresAt}).
		FirstOrCreate(&models.GymQRToken{})
		
	if result.Error != nil {
		return "", result.Error
	}

	return token, nil
}

// ---------------------------------------------------------------------------
// Handler: GET /api/attendance/qr  (tablet side)
// ---------------------------------------------------------------------------

func GetQRToken(c echo.Context) error {
	gymIDCtx := c.Get("gym_id")
	var gymID uint
	switch v := gymIDCtx.(type) {
	case float64:
		gymID = uint(v)
	case uint:
		gymID = v
	default:
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "gym_id not found in token"})
	}

	token, err := rotateToken(gymID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Could not generate QR token"})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"token":      token,
		"expires_at": time.Now().UTC().Add(24 * time.Hour),
	})
}

// ---------------------------------------------------------------------------
// Handler: POST /api/attendance/qr/scan  (member mobile app)
// ---------------------------------------------------------------------------

type ScanQRRequest struct {
	ScannedToken string `json:"scanned_token"`
}

func ScanQRAttendance(c echo.Context) error {
	var req ScanQRRequest
	if err := c.Bind(&req); err != nil || req.ScannedToken == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "scanned_token is required"})
	}

	// Resolve user from JWT.
	userIDCtx := c.Get("user_id")
	var userID uint
	switch v := userIDCtx.(type) {
	case float64:
		userID = uint(v)
	case uint:
		userID = v
	default:
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "Failed to retrieve user ID from token"})
	}

	// Look up the user to get their gym.
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to fetch user"})
	}
	if user.GymID == nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "User is not associated with any gym"})
	}
	gymID := *user.GymID

	// Fetch the active token for this gym.
	var qrToken models.GymQRToken
	if err := database.DB.Where("gym_id = ?", gymID).First(&qrToken).Error; err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "No active QR token found for your gym"})
	}

	// 1. Validate token expiration first
	if time.Now().UTC().After(qrToken.ExpiresAt) {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "QR token has expired"})
	}

	// 2. Validate token value
	if qrToken.Token != req.ScannedToken {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "Invalid QR token"})
	}

	// Prevent duplicate check-in: one attendance entry per user per calendar day.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	var existing models.Attendance
	err := database.DB.
		Where("user_id = ? AND date = ?", userID, today).
		First(&existing).Error
	if err == nil {
		return c.JSON(http.StatusConflict, echo.Map{"error": "Attendance already marked for today"})
	}

	// Create attendance record.
	now := time.Now().UTC()
	attendance := models.Attendance{
		UserID: userID,
		Date:   today,
		TimeIn: now,
		Source: "QR",
	}
	if err := database.DB.Create(&attendance).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to record attendance"})
	}

	return c.JSON(http.StatusCreated, echo.Map{
		"message":    "Attendance marked successfully",
		"attendance": attendance,
	})
}

// ---------------------------------------------------------------------------
// Handler: POST /api/attendance/:userId  (GymAdmin – manual mark)
// ---------------------------------------------------------------------------

func MarkManualAttendance(c echo.Context) error {
	// Resolve the admin's gym from their JWT.
	gymIDCtx := c.Get("gym_id")
	var adminGymID uint
	switch v := gymIDCtx.(type) {
	case float64:
		adminGymID = uint(v)
	case uint:
		adminGymID = v
	default:
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "gym_id not found in token"})
	}

	// Parse target user ID from URL param.
	userIDParam := c.Param("userId")
	parsedID, err := strconv.ParseUint(userIDParam, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid userId"})
	}
	targetUserID := uint(parsedID)

	// Fetch the target user and verify they belong to the admin's gym.
	var targetUser models.User
	if err := database.DB.First(&targetUser, targetUserID).Error; err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "User not found"})
	}
	if targetUser.GymID == nil || *targetUser.GymID != adminGymID {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "User does not belong to your gym"})
	}

	// Prevent duplicate check-in: one attendance entry per user per calendar day.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	var existing models.Attendance
	if err := database.DB.Where("user_id = ? AND date = ?", targetUserID, today).First(&existing).Error; err == nil {
		return c.JSON(http.StatusConflict, echo.Map{"error": "Attendance already marked for today"})
	}

	// Create attendance record.
	now := time.Now().UTC()
	attendance := models.Attendance{
		UserID: targetUserID,
		Date:   today,
		TimeIn: now,
		Source: "Manual",
	}
	if err := database.DB.Create(&attendance).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to record attendance"})
	}

	return c.JSON(http.StatusCreated, echo.Map{
		"message":    "Attendance marked manually",
		"attendance": attendance,
	})
}