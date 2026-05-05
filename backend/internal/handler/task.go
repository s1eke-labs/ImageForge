package handler

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"path/filepath"
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

func (h TaskHandler) Statuses(c echo.Context) error {
	userID := middleware.CurrentUserID(c)
	ids := taskStatusIDs(c.QueryParam("ids"))
	if len(ids) == 0 {
		return c.JSON(http.StatusOK, map[string]any{"tasks": []any{}})
	}
	if len(ids) > 50 {
		return echo.NewHTTPError(http.StatusBadRequest, "up to 50 task ids are allowed")
	}

	var tasks []model.Task
	if err := h.DB.Where("user_id = ? AND id IN ?", userID, ids).Find(&tasks).Error; err != nil {
		return err
	}
	byID := make(map[string]model.Task, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	out := make([]map[string]any, 0, len(tasks))
	for _, id := range ids {
		task, ok := byID[id]
		if !ok {
			continue
		}
		out = append(out, taskStatusResponse(task))
	}
	return c.JSON(http.StatusOK, map[string]any{"tasks": out})
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
	files, err := referenceImageFiles(c)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(files))
	for i, file := range files {
		path, saveErr := service.SaveReferenceImageAt(h.DataDir, task.CreatedAt, task.ID, i, file)
		if saveErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, saveErr.Error())
		}
		paths = append(paths, path)
	}
	setTaskReferenceImages(&task, paths)
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
	sourcePaths := taskReferenceImagePaths(source)
	paths := make([]string, 0, len(sourcePaths))
	for i, sourcePath := range sourcePaths {
		path, err := service.CopyReferenceImageAt(h.DataDir, sourcePath, retry.CreatedAt, retry.ID, i)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		paths = append(paths, path)
	}
	setTaskReferenceImages(&retry, paths)
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

func taskStatusResponse(task model.Task) map[string]any {
	return map[string]any{
		"id":                  task.ID,
		"status":              task.Status,
		"runner_id":           task.RunnerID,
		"result_image_path":   task.ResultImagePath,
		"result_thumb_path":   thumbPath(task.ResultImagePath, "result.jpg"),
		"result_width":        task.ResultWidth,
		"result_height":       task.ResultHeight,
		"result_size_bytes":   task.ResultSizeBytes,
		"duration_seconds":    task.DurationSeconds,
		"error_code":          task.ErrorCode,
		"error_message":       task.ErrorMessage,
		"upstream_status":     task.UpstreamStatus,
		"upstream_updated_at": task.UpstreamUpdatedAt,
		"claimed_at":          task.ClaimedAt,
		"finished_at":         task.FinishedAt,
	}
}

func taskResponse(task model.Task) map[string]any {
	referencePaths := taskReferenceImagePaths(task)
	referenceThumbPaths := taskReferenceThumbPaths(referencePaths)
	resp := map[string]any{
		"id":                    task.ID,
		"prompt":                task.Prompt,
		"size":                  task.Size,
		"quality":               task.Quality,
		"status":                task.Status,
		"reference_image_path":  firstString(referencePaths),
		"reference_thumb_path":  firstString(referenceThumbPaths),
		"reference_image_paths": referencePaths,
		"reference_thumb_paths": referenceThumbPaths,
		"runner_id":             task.RunnerID,
		"result_image_path":     task.ResultImagePath,
		"result_thumb_path":     thumbPath(task.ResultImagePath, "result.jpg"),
		"result_width":          task.ResultWidth,
		"result_height":         task.ResultHeight,
		"result_size_bytes":     task.ResultSizeBytes,
		"duration_seconds":      task.DurationSeconds,
		"error_code":            task.ErrorCode,
		"error_message":         task.ErrorMessage,
		"upstream_response_id":  task.UpstreamResponseID,
		"submission_id":         task.SubmissionID,
		"upstream_status":       task.UpstreamStatus,
		"upstream_updated_at":   task.UpstreamUpdatedAt,
		"created_at":            task.CreatedAt,
		"claimed_at":            task.ClaimedAt,
		"finished_at":           task.FinishedAt,
	}
	return resp
}

func taskStatusIDs(raw string) []string {
	parts := strings.Split(raw, ",")
	ids := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func referenceImageFiles(c echo.Context) ([]*multipart.FileHeader, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid reference image")
	}
	files := make([]*multipart.FileHeader, 0, service.MaxReferenceImages)
	files = append(files, form.File["reference_images"]...)
	files = append(files, form.File["reference_image"]...)
	if len(files) > service.MaxReferenceImages {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "up to 4 reference images are allowed")
	}
	return files, nil
}

func setTaskReferenceImages(task *model.Task, paths []string) {
	task.ReferenceImagePath = firstString(paths)
	task.ReferenceImagePaths = ""
	if len(paths) == 0 {
		return
	}
	data, err := json.Marshal(paths)
	if err == nil {
		task.ReferenceImagePaths = string(data)
	}
}

func taskReferenceImagePaths(task model.Task) []string {
	var paths []string
	if task.ReferenceImagePaths != "" && json.Unmarshal([]byte(task.ReferenceImagePaths), &paths) == nil {
		return compactStrings(paths)
	}
	if task.ReferenceImagePath != "" {
		return []string{task.ReferenceImagePath}
	}
	return []string{}
}

func taskReferenceThumbPaths(paths []string) []string {
	thumbs := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		thumbs = append(thumbs, thumbPath(path, referenceThumbName(path)))
	}
	return thumbs
}

func referenceThumbName(path string) string {
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	if name == "" || ext == "" {
		return "ref.jpg"
	}
	return strings.TrimSuffix(name, ext) + ".jpg"
}

func firstString(values []string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func thumbPath(path, name string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "task_") {
			base := append([]string{}, parts[:i+1]...)
			base = append(base, "thumbs", name)
			return strings.Join(base, "/")
		}
	}
	return ""
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
