package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"imageforge/backend/internal/middleware"
	"imageforge/backend/internal/model"
	"imageforge/backend/internal/service"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type TaskHandler struct {
	DB      *gorm.DB
	DataDir string
}

func (h TaskHandler) List(c echo.Context) error {
	userID := middleware.CurrentUserID(c)
	page := parseInt(c.QueryParam("page"), 1)
	pageSize := parseInt(c.QueryParam("page_size"), 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 50 {
		pageSize = 50
	}
	var total int64
	if err := h.DB.Model(&model.Task{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return err
	}
	var tasks []model.Task
	if err := h.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&tasks).Error; err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"tasks":     taskResponses(tasks),
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h TaskHandler) Create(c echo.Context) error {
	userID := middleware.CurrentUserID(c)
	prompt := strings.TrimSpace(c.FormValue("prompt"))
	if prompt == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "prompt is required")
	}
	size := strings.TrimSpace(c.FormValue("size"))
	if size == "" {
		size = "1024x1024"
	}
	if err := validateRequestedSize(size); err != nil {
		return err
	}
	taskID, err := service.NewID("task")
	if err != nil {
		return err
	}
	task := model.Task{
		ID:        taskID,
		UserID:    userID,
		Prompt:    prompt,
		Size:      size,
		Quality:   "auto",
		Status:    model.TaskStatusPending,
		CreatedAt: time.Now(),
	}
	file, err := c.FormFile("reference_image")
	if err == nil && file != nil {
		path, saveErr := service.SaveReferenceImage(h.DataDir, task.CreatedAt, task.ID, file)
		if saveErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, saveErr.Error())
		}
		task.ReferenceImagePath = path
	} else if err != nil && err != http.ErrMissingFile {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid reference image")
	}
	if err := h.DB.Create(&task).Error; err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, taskResponse(task))
}

func (h TaskHandler) Get(c echo.Context) error {
	var task model.Task
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), middleware.CurrentUserID(c)).First(&task).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "task not found")
	}
	return c.JSON(http.StatusOK, taskResponse(task))
}

func (h TaskHandler) Retry(c echo.Context) error {
	var source model.Task
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), middleware.CurrentUserID(c)).First(&source).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "task not found")
	}
	if source.Status != model.TaskStatusFailed && source.Status != model.TaskStatusCanceled {
		return echo.NewHTTPError(http.StatusBadRequest, "only failed or canceled tasks can be retried")
	}
	taskID, err := service.NewID("task")
	if err != nil {
		return err
	}
	now := time.Now()
	retry := model.Task{
		ID:        taskID,
		UserID:    source.UserID,
		Prompt:    source.Prompt,
		Size:      source.Size,
		Quality:   source.Quality,
		Status:    model.TaskStatusPending,
		CreatedAt: now,
	}
	if source.ReferenceImagePath != "" {
		path, err := service.CopyReferenceImage(h.DataDir, source.ReferenceImagePath, retry.CreatedAt, retry.ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		retry.ReferenceImagePath = path
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&retry).Error; err != nil {
			return err
		}
		return tx.Delete(&source).Error
	}); err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, taskResponse(retry))
}

func taskResponses(tasks []model.Task) []map[string]any {
	out := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, taskResponse(task))
	}
	return out
}

func taskResponse(task model.Task) map[string]any {
	resp := map[string]any{
		"id":                   task.ID,
		"prompt":               task.Prompt,
		"size":                 task.Size,
		"quality":              task.Quality,
		"status":               task.Status,
		"reference_image_path": task.ReferenceImagePath,
		"reference_thumb_path": thumbPath(task.ReferenceImagePath, "ref.jpg"),
		"runner_id":            task.RunnerID,
		"result_image_path":    task.ResultImagePath,
		"result_thumb_path":    thumbPath(task.ResultImagePath, "result.jpg"),
		"result_width":         task.ResultWidth,
		"result_height":        task.ResultHeight,
		"result_size_bytes":    task.ResultSizeBytes,
		"duration_seconds":     task.DurationSeconds,
		"error_code":           task.ErrorCode,
		"error_message":        task.ErrorMessage,
		"upstream_response_id": task.UpstreamResponseID,
		"submission_id":        task.SubmissionID,
		"upstream_status":      task.UpstreamStatus,
		"upstream_updated_at":  task.UpstreamUpdatedAt,
		"created_at":           task.CreatedAt,
		"claimed_at":           task.ClaimedAt,
		"finished_at":          task.FinishedAt,
	}
	return resp
}

func thumbPath(path, name string) string {
	if path == "" {
		return ""
	}
	base := service.ExtractTaskIDFromImagePath(path)
	if base == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		return ""
	}
	return strings.Join([]string{parts[0], parts[1], base, "thumbs", name}, "/")
}

func parseInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return i
}

func validateRequestedSize(value string) error {
	if strings.EqualFold(strings.TrimSpace(value), "auto") {
		return nil
	}
	width, height, err := parseRequestedSize(value)
	if err != nil {
		return err
	}
	if width%16 != 0 || height%16 != 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "width and height must be multiples of 16px")
	}
	longSide := max(width, height)
	shortSide := min(width, height)
	if longSide > 3840 {
		return echo.NewHTTPError(http.StatusBadRequest, "longest side must be <= 3840px")
	}
	if longSide > shortSide*3 {
		return echo.NewHTTPError(http.StatusBadRequest, "aspect ratio must be <= 3:1")
	}
	pixels := int64(width) * int64(height)
	if pixels < 655360 || pixels > 8294400 {
		return echo.NewHTTPError(http.StatusBadRequest, "total pixels must be between 655360 and 8294400")
	}
	return nil
}

func parseRequestedSize(value string) (int, int, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(value)), "x")
	if len(parts) != 2 {
		return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "size must be WIDTHxHEIGHT")
	}
	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || width <= 0 {
		return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "size width is invalid")
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || height <= 0 {
		return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "size height is invalid")
	}
	return width, height, nil
}
