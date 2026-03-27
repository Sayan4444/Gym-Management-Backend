package routes

import (
	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)

func AddonRoutes(e *echo.Echo) {
	api := e.Group("/api")

	api.GET("/addons", handlers.GetAddons)

	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware())

	protected.POST("/addons", handlers.CreateAddon, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.PUT("/addons/:id", handlers.UpdateAddon, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.DELETE("/addons/:id", handlers.DeleteAddon, middleware.RoleScope("SuperAdmin", "GymAdmin"))
}
