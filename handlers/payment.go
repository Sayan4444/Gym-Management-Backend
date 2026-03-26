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
	"io"
	"math"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	razorpay "github.com/razorpay/razorpay-go"
)

type CreateOrderRequest struct {
	Amount     float64 `json:"amount"` // in INR (not paise)
	PaymentFor string  `json:"payment_for"`
}

type VerifyPaymentRequest struct {
	RazorpayOrderID   string `json:"razorpay_order_id"`
	RazorpayPaymentID string `json:"razorpay_payment_id"`
	RazorpaySignature string `json:"razorpay_signature"`
}

func CreateOrder(c echo.Context) error {
	var req CreateOrderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid request fields"})
	}

	userIdContext := c.Get("user_id")
	var userID uint
	if v, ok := userIdContext.(float64); ok {
		userID = uint(v)
	} else if v, ok := userIdContext.(uint); ok {
		userID = v
	} else {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "Failed to retrieve user ID from token"})
	}

	// 1. Create the database record FIRST to get a unique identifier
	payment := models.Payment{
		UserID:     userID,
		Amount:     req.Amount,
		Status:     "Created", // Use "Created" or "Initiated" before Razorpay confirms
		PaymentFor: req.PaymentFor,
	}

	if err := database.DB.Create(&payment).Error; err != nil {
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
		// If Razorpay fails, you might want to update the DB record status to "Failed" here
		database.DB.Model(&payment).Update("Status", "Failed")
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to create Razorpay order", "details": err.Error()})
	}

	// 5. Safely assert the order ID type
	orderIdInterface, ok := body["id"]
	if !ok {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Razorpay response missing ID"})
	}

	orderId, ok := orderIdInterface.(string)
	if !ok {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Razorpay order ID is not a string"})
	}

	// 6. Update the existing DB record with the Razorpay Order ID
	if err := database.DB.Model(&payment).Update("RazorpayOrderID", orderId).Error; err != nil {
		// The order exists in Razorpay, but failed to link in your DB. Log this critically.
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
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid payment signature"})
	}

	// 4. Fetch the payment record
	var payment models.Payment
	if err := database.DB.Where("razorpay_order_id = ?", req.RazorpayOrderID).First(&payment).Error; err != nil {
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
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to update payment status"})
	}

	return c.JSON(http.StatusOK, echo.Map{"message": "Payment verified successfully", "payment": payment})
}

type PaymentWebhookPayload struct {
	Event   string `json:"event"`
	Payload struct {
		Payment struct {
			Entity struct {
				PaymentID      string `json:"paymentId"`
				OrderID string `json:"order_id"`
				Status  string `json:"status"`
			} `json:"entity"`
		} `json:"payment"`
	} `json:"payload"`
}

func HandleWebhook(c echo.Context) error {
	c.Logger().Info("Webhook hit")
	secret := os.Getenv("RAZORPAY_WEBHOOK_SECRET")
	if secret == "" {
		c.Logger().Error("RAZORPAY_WEBHOOK_SECRET is not set. Aborting.")
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Server misconfiguration"})
	}

	signatureHeader := c.Request().Header.Get("X-Razorpay-Signature")
	if signatureHeader == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "Missing signature"})
	}

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Failed to read body"})
	}

	// Verify Signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(bodyBytes)
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	c.Logger().Infof("1. Secret Length: %d", len(secret)) // Ensures secret is loaded and not empty/padded
    c.Logger().Infof("2. Header Signature: '%s'", signatureHeader)
    c.Logger().Infof("3. Expected Signature: '%s'", expectedSignature)
    
    // Log the exact raw body string. The single quotes will help you spot 
    // trailing newlines or spaces.
    c.Logger().Infof("4. Raw Body: '%s'", string(bodyBytes)) 
    // --- DEBUGGING BLOCK END ---

	if subtle.ConstantTimeCompare([]byte(expectedSignature), []byte(signatureHeader)) != 1 {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "Invalid signature"})
	}

	// Parse Payload
	var payload PaymentWebhookPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
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
			c.Logger().Errorf("Failed to update payment status for order %s: %v", orderID, result.Error)
			// Still return 200 so Razorpay doesn't retry, or 500 if you want a retry. 
			// Usually, logging the error and returning 500 is safer for DB failures.
			return c.NoContent(http.StatusInternalServerError)
		}

		if result.RowsAffected > 0 {
			// This block only executes exactly ONCE per order, making it safe for sending emails/receipts
			c.Logger().Infof("Order %s successfully marked as Paid", orderID)
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
