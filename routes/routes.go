package routes

import (
	"github.com/labstack/echo/v4"
)

func SetupRoutes(e *echo.Echo) {
	AuthRoutes(e)
	GymRoutes(e)
	MembershipRoutes(e)
	UserRoutes(e)
	SubscriptionRoutes(e)
	// api := e.Group("/api")

	// // Public Routes
	// api.GET("/gyms", handlers.GetGyms)
	// api.GET("/gyms/:identifier", handlers.GetGym)
	// api.POST("/demo-request", handlers.SubmitDemoRequest)
	
	// // Protected Routes
	// protected := api.Group("")
	// protected.Use(middleware.JWTMiddleware())
	
	// protected.POST("/gym", handlers.AddGym,middleware.RoleScope("SuperAdmin"));
	
	// // Auth (protected)

	// // Member API
	// protected.GET("/users", handlers.GetUsers, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	// protected.PUT("/profile", handlers.UpdateProfile) // allow users to update their own profile
	// protected.PUT("/members/:id", handlers.EditMember, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	// protected.DELETE("/members/:id", handlers.DeleteMember, middleware.RoleScope("SuperAdmin", "GymAdmin"))

	// // Attendance API
	// protected.POST("/attendance", handlers.LogAttendance, middleware.RoleScope("SuperAdmin", "GymAdmin"))

	// // Memberships & Subscriptions API
	// protected.POST("/plans", handlers.CreatePlan, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	// protected.GET("/plans", handlers.GetPlans, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	// protected.POST("/subscriptions", handlers.AssignSubscription, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	// protected.GET("/subscriptions", handlers.GetSubscriptions)

	// // Addons API
	// protected.POST("/addons", handlers.CreateAddon, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	// protected.GET("/addons", handlers.GetAddons)
	// protected.POST("/addons/buy", handlers.BuyAddon)

	// // Dashboard Config
	// protected.GET("/dashboard/stats", handlers.GetDashboardStats, middleware.RoleScope("GymAdmin"))
}
