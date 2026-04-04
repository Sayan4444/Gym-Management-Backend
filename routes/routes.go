package routes

import (
	"os"

	"github.com/labstack/echo/v4"
)

func SetupRoutes(e *echo.Echo) {
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
}
