package middleware

import (
	"net/http"
	"strings"
	"time"

	"imageforge/backend/internal/model"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type JWTClaims struct {
	UserID       uint   `json:"user_id"`
	Username     string `json:"username"`
	TokenVersion int    `json:"token_version"`
	jwt.RegisteredClaims
}

func SignJWT(user model.User, secret string) (string, error) {
	tokenVersion := user.TokenVersion
	if tokenVersion == 0 {
		tokenVersion = 1
	}
	claims := JWTClaims{
		UserID:       user.ID,
		Username:     user.Username,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func JWTAuth(db *gorm.DB, secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tokenString := bearerToken(c.Request().Header.Get("Authorization"))
			if tokenString == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
			}
			claims := new(JWTClaims)
			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid bearer token")
			}
			var user model.User
			if err := db.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid bearer token")
			}
			if claims.TokenVersion != user.TokenVersion {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid bearer token")
			}
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			return next(c)
		}
	}
}

func CurrentUserID(c echo.Context) uint {
	id, _ := c.Get("user_id").(uint)
	return id
}

func CurrentUsername(c echo.Context) string {
	username, _ := c.Get("username").(string)
	return username
}

func bearerToken(header string) string {
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}
