package routes

import (
	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)

func UserAddonRoutes(e *echo.Echo) {
	api := e.Group("/api")
	
	// Unprotected routes
	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware())

	// GymId is automatically inferred from the auth token used to sign in
	protected.GET("/user-addons", handlers.GetUserAddons, middleware.RoleScope("SuperAdmin", "GymAdmin","Trainer","Member"))
	protected.POST("/user-addons", handlers.AssignUserAddon, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.PUT("/user-addons/:id", handlers.UpdateUserAddon, middleware.RoleScope("SuperAdmin", "GymAdmin"))
	protected.DELETE("/user-addons/:id", handlers.DeleteUserAddon, middleware.RoleScope("SuperAdmin", "GymAdmin"))
}
