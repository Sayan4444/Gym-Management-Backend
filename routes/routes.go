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
	MembershipRoutes(e)
	UserRoutes(e)
	SubscriptionRoutes(e)
	PaymentRoutes(e)
	AddonRoutes(e)
	WorkoutPlanRoutes(e)
	AttendanceRoutes(e)
}
