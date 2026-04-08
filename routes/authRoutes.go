package routes

import (
	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)

func AuthRoutes(e *echo.Echo) {
	api := e.Group("/api/auth")

	// Public Routes
	api.POST("/google", handlers.GoogleLogin)
	api.POST("/logout", handlers.Logout)
	
	// Protected Routes
	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware())
	
	// Auth (protected)
	// GET /users?include=gym,subscription,trainer,workout_plan
	protected.GET("/me", handlers.GetMe)
}
