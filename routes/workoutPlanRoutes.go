package routes

import (
	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)

func WorkoutPlanRoutes(e *echo.Echo) {
	protected := e.Group("/api")
	protected.Use(middleware.JWTMiddleware())

	// Create: only a Trainer (who is the member's assigned trainer) can create
	protected.POST("/workout-plan", handlers.CreateWorkoutPlan, middleware.RoleScope("Trainer"))

	// Read: member can see their own plans
	protected.GET("/workout-plan", handlers.GetWorkoutPlans, middleware.RoleScope("Member"))

	// Update: Trainer (own plans), GymAdmin, SuperAdmin
	protected.PUT("/workout-plan/:id", handlers.UpdateWorkoutPlan, middleware.RoleScope("Trainer", "GymAdmin"))

	// Delete: Trainer (own plans), GymAdmin, SuperAdmin
	protected.DELETE("/workout-plan/:id", handlers.DeleteWorkoutPlan, middleware.RoleScope("Trainer", "GymAdmin"))
}
