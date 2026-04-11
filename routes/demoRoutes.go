package routes

import (
	"gym-saas/handlers"

	"github.com/labstack/echo/v4"
)

func DemoRoutes(e *echo.Echo) {
	api := e.Group("/api")

	api.POST("/demo-request", handlers.SubmitDemoRequest)
}
