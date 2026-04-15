package main

import (
	"log"
	"os"
	"strings"

	"gym-saas/database"
	"gym-saas/jobs"
	"gym-saas/routes"
	"gym-saas/utils"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	gommonlog "github.com/labstack/gommon/log"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	database.InitDB()
	utils.InitJWTSecret()
	jobs.StartSubscriptionCron()

	e := echo.New()
	e.Logger.SetLevel(gommonlog.INFO)

	frontendURLs := os.Getenv("FRONTEND_URL")
	if frontendURLs == "" {
		frontendURLs = "http://localhost:3000"
	}

	allowedOrigins := strings.Split(frontendURLs, ",")

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
	}))

	routes.SetupRoutes(e)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	e.Logger.Fatal(e.Start(":" + port))
}
