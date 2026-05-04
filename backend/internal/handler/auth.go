package handler

import (
	"net/http"

	"imageforge/backend/internal/middleware"
	"imageforge/backend/internal/model"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB        *gorm.DB
	JWTSecret string
}

func (h AuthHandler) Login(c echo.Context) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	var user model.User
	if err := h.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid username or password")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid username or password")
	}
	token, err := middleware.SignJWT(user, h.JWTSecret)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"token": token})
}
