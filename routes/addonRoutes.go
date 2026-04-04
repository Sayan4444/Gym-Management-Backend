package routes

import (
	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)

func AddonRoutes(e *echo.Echo) {
	api := e.Group("/api")
	
	// Public Routes
	// View the addons by the gym id - public route
	api.GET("/gyms/:gymId/addons", handlers.GetAddonsByGym)
	
	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware())
	
	// GymId is automatically inferred from the auth token used to sign in
	protected.GET("/addons", handlers.GetAddons) // Viewable by everyone
	protected.POST("/gyms/:gymId/addon", handlers.CreateAddon, middleware.RoleScope("SuperAdmin","GymAdmin"))
	protected.PUT("/gyms/:gymId/addon/:addonId", handlers.UpdateAddon, middleware.RoleScope("SuperAdmin","GymAdmin"))
	protected.DELETE("/gyms/:gymId/addon/:addonId", handlers.DeleteAddon, middleware.RoleScope("SuperAdmin","GymAdmin"))
}
