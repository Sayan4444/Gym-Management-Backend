package routes

import (
	"gym-saas/handlers"

	"github.com/labstack/echo/v4"
)

func DevRoutes(e *echo.Echo) {
	api := e.Group("/api")

	api.POST("/changeRole", handlers.ChangeRole)
}