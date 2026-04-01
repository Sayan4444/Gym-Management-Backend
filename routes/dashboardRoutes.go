package routes

import (
	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)

func DashboardRoutes(e *echo.Echo) {
	api := e.Group("/api/dashboard")
	api.Use(middleware.JWTMiddleware())

	// Super Admin route
	api.GET("/superadmin", handlers.GetSuperAdminDashboardStats, middleware.RoleScope("SuperAdmin"))

	// Admin (GymAdmin) route
	api.GET("/gymadmin", handlers.GetAdminDashboardStats, middleware.RoleScope("GymAdmin"))
}
