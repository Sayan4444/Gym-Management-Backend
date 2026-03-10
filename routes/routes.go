package routes

import (
	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)

func SetupRoutes(e *echo.Echo) {
	api := e.Group("/api")

	// Public Routes
	api.POST("/auth/google", handlers.GoogleLogin)

	// Protected Routes
	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware())

	// Member API
	protected.POST("/members", handlers.CreateMember, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.GET("/members", handlers.GetMembers, middleware.RoleScope("SuperAdmin", "GymAdmin", "Trainer"))

	// Attendance API
	protected.POST("/attendance", handlers.LogAttendance, middleware.RoleScope("SuperAdmin", "GymAdmin"))

	// Memberships & Subscriptions API
	protected.POST("/plans", handlers.CreatePlan, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.GET("/plans", handlers.GetPlans, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.POST("/subscriptions", handlers.AssignSubscription, middleware.RoleScope("SuperAdmin", "GymAdmin"))

	// Dashboard Config
	protected.GET("/dashboard/stats", handlers.GetDashboardStats, middleware.RoleScope("GymAdmin"))
}
