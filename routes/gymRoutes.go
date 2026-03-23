// gym can be only created by super admin
// gym can data is public can be seen by everyone
// gym can be only deleted by super admin
// gym can be only updated by super admin

package routes

import (
	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)

func GymRoutes(e *echo.Echo) {
	api := e.Group("/api")

	// Protected Routes
	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware())
	
	protected.POST("/gym", handlers.AddGym, middleware.RoleScope("SuperAdmin"))
	protected.GET("/gyms", handlers.GetGyms, middleware.RoleScope("SuperAdmin"))
	protected.GET("/gym/:identifier", handlers.GetGym)
	protected.PUT("/gym/:identifier", handlers.UpdateGym,middleware.RoleScope("SuperAdmin","GymAdmin"))
	protected.DELETE("/gym/:identifier", handlers.DeleteGym, middleware.RoleScope("SuperAdmin"))
}