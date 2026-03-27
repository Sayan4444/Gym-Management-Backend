package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"gym-saas/database"
	"gym-saas/models"

	"github.com/labstack/echo/v4"
)



type AssignSubscriptionRequest struct {
	UserID uint `json:"user_id"`
	PlanID uint `json:"plan_id"`
}

func CreateSubscription(userID uint, planID uint) (*models.Subscription, error) {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return nil, errors.New("User not found")
	}

	var plan models.MembershipPlan
	if err := database.DB.First(&plan, planID).Error; err != nil {
		return nil, errors.New("Plan not found")
	}

	if user.GymID == nil || plan.GymID != *user.GymID {
		return nil, errors.New("User and plan do not belong to the same gym")
	}

	startDate := time.Now()
	endDate := startDate.AddDate(0, plan.DurationMonths, 0)

	sub := models.Subscription{
		UserID:    userID,
		PlanID:    plan.ID,
		StartDate: startDate,
		EndDate:   endDate,
		Status:    "Active",
	}

	if err := database.DB.Create(&sub).Error; err != nil {
		return nil, err
	}

	payment := models.Payment{
		UserID:      userID,
		Amount:      plan.Price,
		PaymentDate: startDate,
		Status:      "Paid",
	}
	database.DB.Create(&payment)

	go sendPaymentSuccessEmail(userID, plan.Price, plan.Name)

	return &sub, nil
}

func AssignSubscription(c echo.Context) error {
	var req AssignSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid input"})
	}

	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	if roleRaw := c.Get("role"); roleRaw != nil {
		role := roleRaw.(string)
		if role == "GymAdmin" {
			gymIDRaw := c.Get("gym_id")
			if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only assign subscriptions to users in your gym"})
			}
		}
	}

	sub, err := CreateSubscription(req.UserID, req.PlanID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, sub)
}

func GetSubscriptions(c echo.Context) error {
	var subs []models.Subscription
	
	gymIDRaw := c.Get("gym_id")
	userIDRaw := c.Get("user_id")

	query := database.DB.Model(&models.Subscription{}).Select("subscriptions.*")

	// Filter by gym if it's a GymAdmin
	if gymIDRaw != nil {
		query = query.Joins("JOIN users ON users.id = subscriptions.user_id").
			Where("users.gym_id = ?", uint(gymIDRaw.(float64)))
	}

	// Filter by specific user if user_id is provided in query params (for admin viewing a member)
	userIDParam := c.QueryParam("user_id")
	if userIDParam != "" {
		query = query.Where("subscriptions.user_id = ?", userIDParam)
	} else if userIDRaw != nil && c.Get("role").(string) == "Member" {
		// If logged in as Member, they can only see their own
		query = query.Where("subscriptions.user_id = ?", uint(userIDRaw.(float64)))
	}

	if err := query.Find(&subs).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch subscriptions"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":         len(subs),
		"subscriptions": subs,
	})
}

type UpdateSubscriptionRequest struct {
	PlanID    uint       `json:"plan_id"`
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	Status    string     `json:"status"`
}

func UpdateSubscription(c echo.Context) error {
	id := c.Param("id")
	var sub models.Subscription
	if err := database.DB.First(&sub, id).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Subscription not found"})
	}

	var user models.User
	if err := database.DB.First(&user, sub.UserID).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "User not found"})
	}

	role := c.Get("role").(string)
	if role == "GymAdmin" {
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only update subscriptions for users in your gym"})
		}
	}

	var req UpdateSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if req.PlanID != 0 {
		var plan models.MembershipPlan
		if err := database.DB.First(&plan, req.PlanID).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "New plan not found"})
		}
		if user.GymID == nil || plan.GymID != *user.GymID {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "User and new plan do not belong to the same gym"})
		}
		sub.PlanID = req.PlanID
	}
	if req.StartDate != nil {
		sub.StartDate = *req.StartDate
	}
	if req.EndDate != nil {
		sub.EndDate = *req.EndDate
	}
	if req.Status != "" {
		sub.Status = req.Status
	}

	if err := database.DB.Save(&sub).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update subscription"})
	}

	return c.JSON(http.StatusOK, sub)
}

func DeleteSubscription(c echo.Context) error {
	id := c.Param("id")
	var sub models.Subscription
	if err := database.DB.First(&sub, id).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Subscription not found"})
	}

	var user models.User
	if err := database.DB.First(&user, sub.UserID).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "User not found"})
	}

	role := c.Get("role").(string)
	if role == "GymAdmin" {
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil || user.GymID == nil || uint(gymIDRaw.(float64)) != *user.GymID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "You can only delete subscriptions for users in your gym"})
		}
	}

	if err := database.DB.Delete(&sub).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete subscription"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Subscription deleted successfully"})
}

type RazorpayWebhookPayload struct {
	Event   string `json:"event"`
	Payload struct {
		Payment struct {
			Entity struct {
				Notes struct {
					UserID string `json:"user_id"`
					PlanID string `json:"plan_id"`
				} `json:"notes"`
			} `json:"entity"`
		} `json:"payment"`
	} `json:"payload"`
}

func RazorpayWebhook(c echo.Context) error {
	secret := os.Getenv("RAZORPAY_WEBHOOK_SECRET")
	signatureHeader := c.Request().Header.Get("X-Razorpay-Signature")

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to read body"})
	}

	// Verify signature if secret is provided
	if secret != "" {
		h := hmac.New(sha256.New, []byte(secret))
		h.Write(bodyBytes)
		expectedSignature := hex.EncodeToString(h.Sum(nil))

		if subtle.ConstantTimeCompare([]byte(expectedSignature), []byte(signatureHeader)) != 1 {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid signature"})
		}
	} else {
		// Log warning in development if secret isn't set
		c.Logger().Warn("RAZORPAY_WEBHOOK_SECRET is not set, skipping signature verification")
	}

	var payload RazorpayWebhookPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid payload"})
	}

	if payload.Event == "payment.captured" || payload.Event == "subscription.charged" {
		userIDStr := payload.Payload.Payment.Entity.Notes.UserID
		planIDStr := payload.Payload.Payment.Entity.Notes.PlanID

		userID, _ := strconv.ParseUint(userIDStr, 10, 32)
		planID, _ := strconv.ParseUint(planIDStr, 10, 32)

		if userID > 0 && planID > 0 {
			_, err := CreateSubscription(uint(userID), uint(planID))
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
		}
	}

	return c.NoContent(http.StatusOK)
}
