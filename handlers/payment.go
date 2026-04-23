package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gym-saas/database"
	"gym-saas/models"
	"gym-saas/utils"
	"io"
	"log"
	"math"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	razorpay "github.com/razorpay/razorpay-go"
)

/*
	1. Frontend hits /createOrder.
	2. Backend calls Razorpay API -> gets a unique OrderId.
	3. Backend sends OrderId to the frontend.
	4. Frontend opens the Razorpay UI using the OrderId -> user makes the payment.
	5. Razorpay frontend returns 3 IDs to your frontend: orderId, paymentId, and signature.
		paymentId = unique id returned after successful payment
		signature = HASH("orderId | paymentId",secret)
	6. Frontend sends these 3 IDs to your backend.
	7. Backend uses the orderId, paymentId, and razerPaySecret to recreate the signature.
	8. Backend compares the recreated signature with the signature received from the frontend.
	9. If they match exactly, the payment is verified.
	10.Backend updates the database and responds to the frontend with a success status.
*/

type CreateOrderRequest struct {
	Amount     float64 `json:"amount"` // in INR (not paise)
	PaymentFor string  `json:"payment_for"`
	PlanID     *uint   `json:"plan_id"`  // required when payment_for == "Membership Plan"
	AddonID    *uint   `json:"addon_id"` // required when payment_for == "Add-On"
}

type VerifyPaymentRequest struct {
	RazorpayOrderID   string `json:"razorpay_order_id"`
	RazorpayPaymentID string `json:"razorpay_payment_id"`
	RazorpaySignature string `json:"razorpay_signature"`
}

func CreateOrder(c echo.Context) error {
	var req CreateOrderRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid request fields"})
	}

	userIDRaw := c.Get("user_id")
	if userIDRaw == nil {
		log.Printf("API Error (http.StatusUnauthorized): Failed to retrieve user ID from token")
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "Failed to retrieve user ID from token"})
	}
	userID := uint(userIDRaw.(float64))

	// 1. Fetch user to get their GymID
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to fetch user details"})
	}
	if user.GymID == nil {
		log.Printf("API Error (http.StatusBadRequest): User is not associated with any gym")
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "User is not associated with any gym"})
	}
	gymID := *user.GymID

	// 2. Validate plan_id / addon_id against the user's gym
	switch req.PaymentFor {
	case "Membership Plan":
		if req.PlanID == nil {
			log.Printf("API Error (http.StatusBadRequest): plan_id is required for Membership Plan payments")
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "plan_id is required for Membership Plan payments"})
		}
		var plan models.MembershipPlan
		if err := database.DB.Where("id = ? AND gym_id = ?", *req.PlanID, gymID).First(&plan).Error; err != nil {
			log.Printf("Error: %v", err)
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "Membership plan not found for your gym"})
		}
	case "Add-On":
		if req.AddonID == nil {
			log.Printf("API Error (http.StatusBadRequest): addon_id is required for Add-On payments")
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "addon_id is required for Add-On payments"})
		}
		var addon models.Addon
		if err := database.DB.Where("id = ? AND gym_id = ?", *req.AddonID, gymID).First(&addon).Error; err != nil {
			log.Printf("Error: %v", err)
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "Add-on not found for your gym"})
		}
	default:
		log.Printf("API Error (http.StatusBadRequest): Invalid payment_for value. Must be 'Membership Plan' or 'Add-On'")
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid payment_for value. Must be 'Membership Plan' or 'Add-On'"})
	}

	// 3. Validate minimum amount
	if req.Amount < 1.00 {
		log.Printf("API Error (http.StatusBadRequest): Amount must be at least 1 INR")
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Amount must be at least 1 INR (100 paise)"})
	}

	// 4. Create the database record FIRST to get a unique identifier
	payment := models.Payment{
		UserID:     userID,
		Amount:     req.Amount,
		Status:     "Created", // Use "Created" or "Initiated" before Razorpay confirms
		PaymentFor: req.PaymentFor,
		PlanID:     req.PlanID,
		AddonID:    req.AddonID,
	}

	if err := database.DB.Create(&payment).Error; err != nil {

		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to initialize payment record"})
	}

	// 2. Use the database ID as the receipt.
	// Assuming your DB model uses an auto-incrementing uint or a UUID for its primary key (ID).
	receiptID := fmt.Sprintf("rcpt_%v", payment.ID)

	// 3. Safely convert float to paise using math.Round
	amountInPaise := int(math.Round(req.Amount * 100))

	data := map[string]interface{}{
		"amount":   amountInPaise,
		"currency": "INR",
		"receipt":  receiptID,
	}

	// 4. Create the order
	razorpayClient := razorpay.NewClient(os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_KEY_SECRET"))
	body, err := razorpayClient.Order.Create(data, nil)
	if err != nil {
		log.Printf("Error: %v", err)
		// If Razorpay fails, you might want to update the DB record status to "Failed" here
		database.DB.Model(&payment).Update("Status", "Failed")
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to create Razorpay order", "details": err.Error()})
	}

	// 5. Safely assert the order ID type
	orderIdInterface, ok := body["id"]
	if !ok {
		log.Printf("API Error (http.StatusInternalServerError): Razorpay response missing ID")
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Razorpay response missing ID"})
	}

	orderId, ok := orderIdInterface.(string)
	if !ok {
		log.Printf("API Error (http.StatusInternalServerError): Razorpay order ID is not a string")
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Razorpay order ID is not a string"})
	}

	// 6. Update the existing DB record with the Razorpay Order ID
	if err := database.DB.Model(&payment).Update("RazorpayOrderID", orderId).Error; err != nil {
		log.Printf("Error: %v", err)
		// The order exists in Razorpay, but failed to link in your DB. Log this critically.
		log.Printf("API Error (http.StatusInternalServerError): Order created but failed to link to database")
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Order created but failed to link to database"})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"order_id": orderId,
		"amount":   req.Amount,
		"currency": "INR",
	})
}

func VerifyPayment(c echo.Context) error {
	var req VerifyPaymentRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid request fields"})
	}

	// 1. Fetch Razorpay Secret
	secret := os.Getenv("RAZORPAY_KEY_SECRET")

	// 2. Generate Expected Signature
	signatureData := req.RazorpayOrderID + "|" + req.RazorpayPaymentID
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signatureData))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	// 3. SECURE COMPARISON: Use constant-time compare to prevent timing attacks
	expectedSignatureBytes := []byte(expectedSignature)
	providedSignatureBytes := []byte(req.RazorpaySignature)

	if subtle.ConstantTimeCompare(expectedSignatureBytes, providedSignatureBytes) != 1 {
		log.Printf("API Error (http.StatusBadRequest): Invalid payment signature")
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid payment signature"})
	}

	// 4. Fetch the payment record
	var payment models.Payment
	if err := database.DB.Where("razorpay_order_id = ?", req.RazorpayOrderID).First(&payment).Error; err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusNotFound, echo.Map{"error": "Payment record not found"})
	}

	// 5. IDEMPOTENCY CHECK: Prevent double-processing
	if payment.Status == "Paid" {
		return c.JSON(http.StatusOK, echo.Map{"message": "Payment already verified", "payment": payment})
	}

	// 6. Update Payment status
	payment.Status = "Paid"
	payment.RazorpayPaymentID = req.RazorpayPaymentID
	payment.RazorpaySignature = req.RazorpaySignature

	if err := database.DB.Save(&payment).Error; err != nil {

		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to update payment status"})
	}

	switch payment.PaymentFor {
	case "Membership Plan":
		if payment.PlanID != nil {
			_, _, _ = AssignSubscriptionLogic(payment.UserID, *payment.PlanID)
		}
	case "Add-On":
		if payment.AddonID != nil {
			_, _, _ = AssignUserAddonLogic(payment.UserID, *payment.AddonID)
		}
	}

	go sendPaymentSuccessEmail(payment.UserID, payment.Amount, payment.PaymentFor)

	return c.JSON(http.StatusOK, echo.Map{"message": "Payment verified successfully", "payment": payment})
}

type PaymentWebhookPayload struct {
	Event   string `json:"event"`
	Payload struct {
		Payment struct {
			Entity struct {
				PaymentID string `json:"paymentId"`
				OrderID   string `json:"order_id"`
				Status    string `json:"status"`
			} `json:"entity"`
		} `json:"payment"`
	} `json:"payload"`
}

func HandleWebhook(c echo.Context) error {
	c.Logger().Info("Webhook hit")
	secret := os.Getenv("RAZORPAY_WEBHOOK_SECRET")
	if secret == "" {
		c.Logger().Error("RAZORPAY_WEBHOOK_SECRET is not set. Aborting.")
		log.Printf("API Error (http.StatusInternalServerError): Server misconfiguration")
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Server misconfiguration"})
	}

	signatureHeader := c.Request().Header.Get("X-Razorpay-Signature")
	if signatureHeader == "" {
		log.Printf("API Error (http.StatusUnauthorized): Missing signature")
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "Missing signature"})
	}

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Failed to read body"})
	}

	// Verify Signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(bodyBytes)
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(expectedSignature), []byte(signatureHeader)) != 1 {
		log.Printf("API Error (http.StatusUnauthorized): Invalid signature")
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "Invalid signature"})
	}

	// Parse Payload
	var payload PaymentWebhookPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		log.Printf("Error: %v", err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid payload"})
	}

	// Route based on Event
	switch payload.Event {
	case "payment.captured": // Consider using "order.paid" if utilizing Razorpay Orders
		orderID := payload.Payload.Payment.Entity.OrderID
		paymentID := payload.Payload.Payment.Entity.PaymentID

		// Atomic update: Only update if the current status is NOT "Paid"
		result := database.DB.Model(&models.Payment{}).
			Where("razorpay_order_id = ? AND status != ?", orderID, "Paid").
			Updates(map[string]interface{}{
				"status":              "Paid",
				"razorpay_payment_id": paymentID,
			})

		if result.Error != nil {

			log.Printf("Error: %v", result.Error)
			c.Logger().Errorf("Failed to update payment status for order %s: %v", orderID, result.Error)
			// Still return 200 so Razorpay doesn't retry, or 500 if you want a retry.
			// Usually, logging the error and returning 500 is safer for DB failures.
			return c.NoContent(http.StatusInternalServerError)
		}

		if result.RowsAffected > 0 {
			// This block only executes exactly ONCE per order, making it safe for sending emails/receipts
			c.Logger().Infof("Order %s successfully marked as Paid", orderID)

			var fullPayment models.Payment
			if err := database.DB.Where("razorpay_order_id = ?", orderID).First(&fullPayment).Error; err == nil {
				switch fullPayment.PaymentFor {
				case "Membership Plan":
					if fullPayment.PlanID != nil {
						_, _, _ = AssignSubscriptionLogic(fullPayment.UserID, *fullPayment.PlanID)
					}
				case "Add-On":
					if fullPayment.AddonID != nil {
						_, _, _ = AssignUserAddonLogic(fullPayment.UserID, *fullPayment.AddonID)
					}
				}
				go sendPaymentSuccessEmail(fullPayment.UserID, fullPayment.Amount, fullPayment.PaymentFor)
			}
		}

	case "payment.failed":
		orderID := payload.Payload.Payment.Entity.OrderID
		paymentID := payload.Payload.Payment.Entity.PaymentID

		// Atomic update: Only update to Failed if it hasn't already been marked as Paid
		database.DB.Model(&models.Payment{}).
			Where("razorpay_order_id = ? AND status != ?", orderID, "Paid").
			Updates(map[string]interface{}{
				"status":              "Failed",
				"razorpay_payment_id": paymentID,
			})
	}

	return c.NoContent(http.StatusOK)
}

func sendPaymentSuccessEmail(userID uint, amount float64, paymentFor string) {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err == nil {
		subject := "Payment Successful & Subscription Confirmed"
		body := fmt.Sprintf("Dear %s,\n\nYour payment of ₹%.2f for %s has been successfully processed.\nYour subscription is now active!\n\nThank you for choosing us.\n\nBest Regards,\nGym Management Team", user.Name, amount, paymentFor)
		go utils.SendEmail(user.Email, subject, body)
	}
}

func GetPayments(c echo.Context) error {
	var payments []models.Payment
	query := database.DB.Model(&models.Payment{}).Joins("JOIN users ON users.id = payments.user_id")

	// Role-based filtering
	roleRaw := c.Get("role")
	role, ok := roleRaw.(string)
	if !ok {
		log.Printf("API Error (http.StatusUnauthorized): Unauthorized")
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "Unauthorized"})
	}

	switch role {
	case "SuperAdmin":
		// They can fetch any users payment info
		if gymID := c.QueryParam("gym_id"); gymID != "" {
			query = query.Where("users.gym_id = ?", gymID)
		}
	case "GymAdmin":
		gymIDRaw := c.Get("gym_id")
		if gymIDRaw == nil {
			log.Printf("API Error (http.StatusForbidden): Gym ID required")
			return c.JSON(http.StatusForbidden, echo.Map{"error": "Gym ID required"})
		}
		query = query.Where("users.gym_id = ?", uint(gymIDRaw.(float64)))
	default: // Trainer, Member
		// They can only get their own payment info
		userIDRaw := c.Get("user_id")
		if userIDRaw == nil {
			log.Printf("API Error (http.StatusUnauthorized): Unauthorized")
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "Unauthorized"})
		}
		query = query.Where("payments.user_id = ?", uint(userIDRaw.(float64)))
	}

	// Query Filters
	if status := c.QueryParam("status"); status != "" && status != "all" {
		query = query.Where("payments.status = ?", status)
	}
	if search := c.QueryParam("search"); search != "" {
		query = query.Where("users.name ILIKE ?", "%"+search+"%")
	}
	if targetUserID := c.QueryParam("user_id"); targetUserID != "" {
		query = query.Where("payments.user_id = ?", targetUserID)
	}

	if err := query.Order("payments.created_at DESC").Find(&payments).Error; err != nil {

		log.Printf("Error: %v", err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to fetch payments"})
	}

	// Let's create an anonymous struct to embed user name so the frontend gets it
	type PaymentWithUser struct {
		models.Payment
		UserName string `json:"user_name"`
	}

	var result []PaymentWithUser
	for _, p := range payments {
		var user models.User
		database.DB.First(&user, p.UserID)
		result = append(result, PaymentWithUser{
			Payment:  p,
			UserName: user.Name,
		})
	}

	return c.JSON(http.StatusOK, echo.Map{"count": len(result), "payments": result})
}
