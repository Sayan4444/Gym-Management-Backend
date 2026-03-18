package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"gym-saas/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

func JWTMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var tokenString string

			// 1. Try to read token from the auth_token cookie
			cookie, err := c.Cookie("auth_token")
			if err == nil && cookie.Value != "" {
				tokenString = cookie.Value
			}

			// 2. Fall back to Authorization header
			if tokenString == "" {
				authHeader := c.Request().Header.Get("Authorization")
				if authHeader != "" {
					tokenString = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if tokenString == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Missing token"})
			}

			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method")
				}
				return utils.JWTSecret, nil
			})

			if err != nil || !token.Valid {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid claims"})
			}

			c.Set("user_id", claims["user_id"])
			c.Set("role", claims["role"])
			if gymID, ok := claims["gym_id"]; ok && gymID != nil {
				c.Set("gym_id", gymID)
			}

			return next(c)
		}
	}
}

func RoleScope(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userRole := c.Get("role").(string)
			if userRole == "SuperAdmin" {
				return next(c)
			}
			for _, role := range roles {
				if userRole == role {
					return next(c)
				}
			}
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
		}
	}
}
