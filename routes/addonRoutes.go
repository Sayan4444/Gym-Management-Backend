package routes

import (
	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)

func AddonRoutes(e *echo.Echo) {
	api := e.Group("/api")
	
	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware())

	protected.POST("/addons", handlers.CreateAddon, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.GET("/addons", handlers.GetAddons)
	protected.POST("/addons/buy", handlers.BuyAddon)
}
