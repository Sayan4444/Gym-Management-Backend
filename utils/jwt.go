package utils

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var JWTSecret []byte

func InitJWTSecret() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "defaultsecret"
	}
	JWTSecret = []byte(secret)
}

func GenerateToken(userID uint, role string, gymID *uint) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(time.Hour * 72).Unix(),
	}

	if gymID != nil {
		claims["gym_id"] = *gymID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}
