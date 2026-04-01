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

	// Read: users can see plans according to their roles
	protected.GET("/workout-plan", handlers.GetWorkoutPlans)

	// Update: Trainer (own plans), GymAdmin, SuperAdmin
	protected.PUT("/workout-plan/:id", handlers.UpdateWorkoutPlan, middleware.RoleScope("Trainer", "GymAdmin"))

	// Delete: Trainer (own plans), GymAdmin, SuperAdmin
	protected.DELETE("/workout-plan/:id", handlers.DeleteWorkoutPlan, middleware.RoleScope("Trainer", "GymAdmin"))
}
