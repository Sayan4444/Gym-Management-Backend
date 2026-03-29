package routes

import (
	"os"

	"gym-saas/handlers"
	"gym-saas/middleware"

	"github.com/labstack/echo/v4"
)

func AttendanceRoutes(e *echo.Echo) {
	api := e.Group("/api/attendance")

	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware())

	isDev := os.Getenv("ENV") == "development"

	if isDev {
		// Dev: skip role checks – any valid JWT can hit these endpoints.
		protected.GET("/qr", handlers.GetQRToken)
		protected.POST("/qr/scan", handlers.ScanQRAttendance)
	} else {
		// Prod: enforce role scopes.
		protected.GET("/qr", handlers.GetQRToken, middleware.RoleScope("GymAdmin"))
		protected.POST("/qr/scan", handlers.ScanQRAttendance, middleware.RoleScope("Member", "Trainer"))
	}
	protected.POST("/:userId", handlers.MarkManualAttendance, middleware.RoleScope("GymAdmin"))
}
