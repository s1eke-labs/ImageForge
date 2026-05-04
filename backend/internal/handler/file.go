package handler

import (
	"net/http"

	"imageforge/backend/internal/middleware"
	"imageforge/backend/internal/model"
	"imageforge/backend/internal/service"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type FileHandler struct {
	DB      *gorm.DB
	DataDir string
}

func (h FileHandler) UserFile(c echo.Context) error {
	relPath := c.Param("*")
	taskID := service.ExtractTaskIDFromImagePath(relPath)
	if taskID == "" {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}
	var count int64
	if err := h.DB.Model(&model.Task{}).Where("id = ? AND user_id = ?", taskID, middleware.CurrentUserID(c)).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}
	return h.sendFile(c, relPath)
}

func (h FileHandler) RunnerFile(c echo.Context) error {
	runner := middleware.CurrentRunner(c)
	relPath := c.Param("*")
	taskID := service.ExtractTaskIDFromImagePath(relPath)
	if taskID == "" {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}
	var count int64
	if err := h.DB.Model(&model.Task{}).Where("id = ? AND runner_id = ?", taskID, runner.ID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}
	return h.sendFile(c, relPath)
}

func (h FileHandler) sendFile(c echo.Context, relPath string) error {
	abs, err := service.ResolveSafePath(h.DataDir, relPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid file path")
	}
	return c.File(abs)
}
