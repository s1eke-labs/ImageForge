package middleware

import (
	"net/http"
	"strings"

	"imageforge/backend/internal/model"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func RunnerAuth(db *gorm.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := bearerToken(c.Request().Header.Get("Authorization"))
			if len(token) < 8 || !strings.HasPrefix(token, "rtkn_") {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid runner token")
			}
			prefix := token[:8]
			var runners []model.Runner
			if err := db.Where("token_prefix = ?", prefix).Find(&runners).Error; err != nil {
				return err
			}
			for _, runner := range runners {
				if bcrypt.CompareHashAndPassword([]byte(runner.TokenHash), []byte(token)) == nil {
					c.Set("runner", runner)
					return next(c)
				}
			}
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid runner token")
		}
	}
}

func CurrentRunner(c echo.Context) model.Runner {
	runner, _ := c.Get("runner").(model.Runner)
	return runner
}
