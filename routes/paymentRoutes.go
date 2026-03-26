package routes

import (
	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)

func PaymentRoutes(e *echo.Echo) {
	paymentGroup := e.Group("/api/payment")

	paymentGroup.POST("/create-order", handlers.CreateOrder, middleware.JWTMiddleware(), middleware.RoleScope("Member"))
	paymentGroup.POST("/verify", handlers.VerifyPayment)
	paymentGroup.POST("/webhook", handlers.HandleWebhook)
}
