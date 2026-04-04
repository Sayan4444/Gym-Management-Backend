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

	// GymId is automatically inferred from the auth token used to sign in
	// /subscriptions?user_id=45
	protected.GET("/subscriptions", handlers.GetSubscriptions, middleware.RoleScope("SuperAdmin", "GymAdmin","Trainer","Member"))
	protected.POST("/subscriptions", handlers.AssignSubscription, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.PUT("/subscriptions/:id", handlers.UpdateSubscription, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.DELETE("/subscriptions/:id", handlers.DeleteSubscription, middleware.RoleScope("SuperAdmin", "GymAdmin"))
}
