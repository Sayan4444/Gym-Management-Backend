package routes

import (
	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)
// super-admin can get all the users and update any details about the user
// admin can get all the users of their gym and update any details
// trainer can get all the user of their gym under him but cant update any details
// member can only update their own information


func UserRoutes(e *echo.Echo) {
	api := e.Group("/api")

	// Protected Routes
	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware())
	
	// Member API
	protected.GET("/users", handlers.GetUsers, middleware.RoleScope("SuperAdmin", "GymAdmin", "Trainer"))
	protected.PUT("/users/:id", handlers.UpdateProfile, middleware.RoleScope("SuperAdmin", "GymAdmin", "Trainer", "Member")) // allow users to update their own profile
	protected.DELETE("/users/:id", handlers.DeleteProfile, middleware.RoleScope("SuperAdmin", "GymAdmin", "Trainer", "Member"))
 
}
