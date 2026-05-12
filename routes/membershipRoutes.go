package routes

import (
	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)

func MembershipRoutes(e *echo.Echo) {
	api := e.Group("/api")

	// Public Routes
	// View the membership plan by the gym id - public route
	api.GET("/gyms/:gymId/memberships", handlers.GetMembershipPlansByGym)

	// Protected Routes
	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware())

	protected.GET("/memberships", handlers.GetMembershipPlans) // Viewable by everyone
	// Membership can only be created and updated by the gymAdmin
	protected.POST("/gyms/:gymId/memberships", handlers.CreateMembershipPlan, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.PUT("/gyms/:gymId/memberships/:membershipId", handlers.UpdateMembershipPlan, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.DELETE("/gyms/:gymId/memberships/:membershipId", handlers.DeleteMembershipPlan, middleware.RoleScope("SuperAdmin", "GymAdmin"))

	// Plan Addon management (addons included in a plan with a frequency)
	protected.POST("/gyms/:gymId/memberships/:membershipId/addons", handlers.AddPlanAddon, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.PUT("/gyms/:gymId/memberships/:membershipId/addons/:planAddonId", handlers.UpdatePlanAddon, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.DELETE("/gyms/:gymId/memberships/:membershipId/addons/:planAddonId", handlers.RemovePlanAddon, middleware.RoleScope("SuperAdmin", "GymAdmin"))
}
