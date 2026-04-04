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
	
	/*
		GET /users
		GET /users?gym_id=5
		GET /users?role=Trainer	
		GET /users?is_premium=true - premium should be the name of the plan
		GET /users?subscription_status= - search by subscription status
		GET /users?search=alex - all users with name, email or phone with alex
	*/
	protected.GET("/users", handlers.GetUsers, middleware.RoleScope("SuperAdmin", "GymAdmin", "Trainer"))

	// test these
	protected.PUT("/users/:id", handlers.UpdateProfile) // allow users to update their own profile
	protected.DELETE("/users/:id", handlers.DeleteProfile)
 
}
