package routes

import (
	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)

func SubscriptionRoutes(e *echo.Echo) {
	api := e.Group("/api")
	
	// Unprotected routes
	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware())

	protected.POST("/subscriptions", handlers.AssignSubscription, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.GET("/subscriptions", handlers.GetSubscriptions, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.PUT("/subscriptions/:id", handlers.UpdateSubscription, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.DELETE("/subscriptions/:id", handlers.DeleteSubscription, middleware.RoleScope("SuperAdmin", "GymAdmin"))
}
