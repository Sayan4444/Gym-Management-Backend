package routes

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

func SetupRoutes(e *echo.Echo) {
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	if os.Getenv("ENV") == "development" {
		DevRoutes(e)
	}
	AuthRoutes(e)
	GymRoutes(e)
	UserRoutes(e)
	MembershipRoutes(e)
	AddonRoutes(e)
	SubscriptionRoutes(e)
	UserAddonRoutes(e)
	PaymentRoutes(e)
	WorkoutPlanRoutes(e)
	AttendanceRoutes(e)
	DashboardRoutes(e)
	DemoRoutes(e)
}
