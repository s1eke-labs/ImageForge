package handler

import (
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"os"
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

type RunnerHandler struct {
	DB      *gorm.DB
	DataDir string
}

func (h RunnerHandler) List(c echo.Context) error {
	var runners []model.Runner
	if err := h.DB.Order("created_at DESC").Find(&runners).Error; err != nil {
		return err
	}
	now := time.Now()
	resp := make([]map[string]any, 0, len(runners))
	for _, runner := range runners {
		resp = append(resp, runnerResponse(runner, now))
	}
	return c.JSON(http.StatusOK, map[string]any{"runners": resp})
}

func (h RunnerHandler) Delete(c echo.Context) error {
	res := h.DB.Where("id = ?", c.Param("id")).Delete(&model.Runner{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "runner not found")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h RunnerHandler) Register(c echo.Context) error {
	runner := middleware.CurrentRunner(c)
	var req struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	_ = c.Bind(&req)
	now := time.Now()
	updates := map[string]any{
		"status":            model.RunnerStatusOnline,
		"last_heartbeat_at": &now,
	}
	if strings.TrimSpace(req.Version) != "" {
		updates["version"] = strings.TrimSpace(req.Version)
	}
	if err := h.DB.Model(&model.Runner{}).Where("id = ?", runner.ID).Updates(updates).Error; err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"runner_id":                  runner.ID,
		"heartbeat_interval_seconds": 30,
		"poll_interval_seconds":      10,
	})
}

func (h RunnerHandler) Heartbeat(c echo.Context) error {
	runner := middleware.CurrentRunner(c)
	if c.Param("runner_id") != runner.ID {
		return echo.NewHTTPError(http.StatusForbidden, "runner_id does not match token")
	}
	var req struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	_ = c.Bind(&req)
	status := req.Status
	switch status {
	case model.RunnerStatusBusy, model.RunnerStatusOffline:
	default:
		status = model.RunnerStatusOnline
	}
	now := time.Now()
	if err := h.DB.Model(&model.Runner{}).Where("id = ?", runner.ID).Updates(map[string]any{
		"status":            status,
		"version":           strings.TrimSpace(req.Version),
		"last_heartbeat_at": &now,
	}).Error; err != nil {
		return err
	}
	if status == model.RunnerStatusOffline {
		if err := service.FailStalledTasks(h.DB, now); err != nil {
			return err
		}
	}
	return c.NoContent(http.StatusOK)
}

func (h RunnerHandler) Claim(c echo.Context) error {
	runner := middleware.CurrentRunner(c)
	if c.Param("runner_id") != runner.ID {
		return echo.NewHTTPError(http.StatusForbidden, "runner_id does not match token")
	}
	var claimedTask map[string]any
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var task model.Task
		if err := tx.Where("status = ?", model.TaskStatusPending).
			Order("created_at ASC").
			First(&task).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		response, err := runnerTaskResponse(h.DataDir, task)
		if err != nil {
			return err
		}
		now := time.Now()
		task.Status = model.TaskStatusClaimed
		task.RunnerID = &runner.ID
		task.ClaimedAt = &now
		if err := tx.Save(&task).Error; err != nil {
			return err
		}
		claimedTask = response
		return nil
	})
	if err != nil {
		return err
	}
	if claimedTask == nil {
		return c.JSON(http.StatusOK, map[string]any{"tasks": []any{}})
	}
	if err := setRunnerStatus(h.DB, runner.ID, model.RunnerStatusBusy, time.Now()); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"tasks": []any{claimedTask}})
}

func (h RunnerHandler) Result(c echo.Context) error {
	runner := middleware.CurrentRunner(c)
	taskID := c.Param("task_id")
	var task model.Task
	if err := h.DB.Where("id = ? AND runner_id = ?", taskID, runner.ID).First(&task).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "task not found")
	}
	if model.IsTerminalTaskStatus(task.Status) {
		return c.JSON(http.StatusOK, taskResponse(task))
	}
	result, err := parseRunnerResult(c)
	if err != nil {
		return err
	}
	now := time.Now()
	if result.Status == model.TaskStatusFailed {
		task.Status = model.TaskStatusFailed
		task.ErrorCode = result.ErrorCode
		task.ErrorMessage = result.ErrorMessage
		task.FinishedAt = &now
	} else if result.Status == model.TaskStatusSucceeded {
		if result.Image == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "image file is required")
		}
		meta, err := service.SaveResultFile(h.DataDir, task.CreatedAt, task.ID, result.Image)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		task.Status = model.TaskStatusSucceeded
		task.ResultImagePath = meta.Path
		task.ResultWidth = firstNonZero(result.Width, meta.Width)
		task.ResultHeight = firstNonZero(result.Height, meta.Height)
		task.ResultSizeBytes = firstNonZero64(result.SizeBytes, meta.SizeBytes)
		task.DurationSeconds = result.DurationSeconds
		task.UpstreamResponseID = result.UpstreamResponseID
		task.FinishedAt = &now
	} else {
		return echo.NewHTTPError(http.StatusBadRequest, "status must be succeeded or failed")
	}
	task.RunnerID = &runner.ID
	if err := h.DB.Save(&task).Error; err != nil {
		return err
	}
	if err := setRunnerStatus(h.DB, runner.ID, model.RunnerStatusOnline, now); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, taskResponse(task))
}

func (h RunnerHandler) Status(c echo.Context) error {
	runner := middleware.CurrentRunner(c)
	taskID := c.Param("task_id")
	var report runnerStatusReport
	if err := c.Bind(&report); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if report.SourceTaskID != "" && report.SourceTaskID != taskID {
		return echo.NewHTTPError(http.StatusBadRequest, "source_task_id does not match path")
	}
	if !isRunnerReportedStatus(report.Status) {
		return echo.NewHTTPError(http.StatusBadRequest, "status must be queued, leased, running, succeeded, failed, or canceled")
	}

	var task model.Task
	if err := h.DB.Where("id = ?", taskID).First(&task).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "task not found")
	}
	if task.RunnerID != nil && *task.RunnerID != runner.ID {
		return echo.NewHTTPError(http.StatusNotFound, "task not found")
	}
	if task.RunnerID == nil && report.Status != runnerReportStatusQueued {
		return echo.NewHTTPError(http.StatusNotFound, "task not found")
	}

	incomingUpdatedAt := unixTimestamp(report.UpdatedAt)
	if incomingUpdatedAt != nil && task.UpstreamUpdatedAt != nil && incomingUpdatedAt.Before(*task.UpstreamUpdatedAt) {
		return c.JSON(http.StatusOK, taskResponse(task))
	}
	if model.IsTerminalTaskStatus(task.Status) {
		return c.JSON(http.StatusOK, taskResponse(task))
	}

	now := time.Now()
	upstreamUpdatedAt := &now
	if incomingUpdatedAt != nil {
		upstreamUpdatedAt = incomingUpdatedAt
	}
	updates := map[string]any{
		"upstream_status":     report.Status,
		"upstream_updated_at": upstreamUpdatedAt,
	}
	if report.SubmissionID != "" && task.SubmissionID == "" {
		updates["submission_id"] = report.SubmissionID
	}

	switch report.Status {
	case runnerReportStatusQueued:
		if task.Status == "" {
			updates["status"] = model.TaskStatusPending
		}
	case runnerReportStatusLeased, runnerReportStatusRunning:
		updates["status"] = model.TaskStatusClaimed
		updates["runner_id"] = runner.ID
		if task.ClaimedAt == nil {
			updates["claimed_at"] = firstNonNilTime(unixTimestamp(report.StartedAt), &now)
		}
	case model.TaskStatusFailed, model.TaskStatusCanceled:
		updates["status"] = report.Status
		updates["error_code"] = firstNonEmpty(report.ErrorCode, report.Error.Code)
		updates["error_message"] = firstNonEmpty(report.ErrorMessage, report.Error.Message)
		updates["finished_at"] = firstNonNilTime(incomingUpdatedAt, &now)
	case model.TaskStatusSucceeded:
		// Result upload remains authoritative for success because it supplies the image file.
	}

	if err := h.DB.Model(&model.Task{}).Where("id = ?", task.ID).Updates(updates).Error; err != nil {
		return err
	}
	if err := h.DB.Where("id = ?", taskID).First(&task).Error; err != nil {
		return err
	}
	switch report.Status {
	case runnerReportStatusLeased, runnerReportStatusRunning:
		if err := setRunnerStatus(h.DB, runner.ID, model.RunnerStatusBusy, now); err != nil {
			return err
		}
	case model.TaskStatusFailed, model.TaskStatusCanceled, model.TaskStatusSucceeded:
		if err := setRunnerStatus(h.DB, runner.ID, model.RunnerStatusOnline, now); err != nil {
			return err
		}
	}
	return c.JSON(http.StatusOK, taskResponse(task))
}

func setRunnerStatus(tx *gorm.DB, runnerID, status string, now time.Time) error {
	return tx.Model(&model.Runner{}).Where("id = ?", runnerID).Updates(map[string]any{
		"status":            status,
		"last_heartbeat_at": &now,
	}).Error
}

func runnerResponse(runner model.Runner, now time.Time) map[string]any {
	return map[string]any{
		"id":                runner.ID,
		"name":              runner.Name,
		"status":            runner.EffectiveStatus(now),
		"version":           runner.Version,
		"last_heartbeat_at": runner.LastHeartbeatAt,
		"created_at":        runner.CreatedAt,
	}
}

func runnerTaskResponse(dataDir string, task model.Task) (map[string]any, error) {
	references := []map[string]any{}
	if task.ReferenceImagePath != "" {
		abs, err := service.ResolveSafePath(dataDir, task.ReferenceImagePath)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, err
		}
		references = append(references, map[string]any{
			"b64_json":  base64.StdEncoding.EncodeToString(data),
			"file_name": filepath.Base(task.ReferenceImagePath),
			"mime_type": referenceImageMIMEType(data, task.ReferenceImagePath),
		})
	}
	return map[string]any{
		"id":              task.ID,
		"source_task_id":  task.ID,
		"idempotency_key": task.ID,
		"queue":           "default",
		"priority":        "normal",
		"payload": map[string]any{
			"prompt":           task.Prompt,
			"model":            nil,
			"size":             task.Size,
			"quality":          task.Quality,
			"n":                1,
			"reference_images": references,
		},
		"requester": "imageforge",
		"trace_id":  task.ID,
	}, nil
}

func referenceImageMIMEType(data []byte, relPath string) string {
	contentType := http.DetectContentType(data)
	if strings.HasPrefix(contentType, "image/") {
		return contentType
	}
	switch strings.ToLower(filepath.Ext(relPath)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

type runnerResult struct {
	Status             string
	Image              *multipart.FileHeader
	Width              int
	Height             int
	SizeBytes          int64
	DurationSeconds    float64
	ErrorCode          string
	ErrorMessage       string
	UpstreamResponseID string
}

type runnerResultMetadata struct {
	SourceTaskID       string  `json:"source_task_id"`
	Status             string  `json:"status"`
	Width              int     `json:"width"`
	Height             int     `json:"height"`
	SizeBytes          int64   `json:"size_bytes"`
	DurationSeconds    float64 `json:"duration_seconds"`
	UpstreamResponseID string  `json:"upstream_response_id"`
	ErrorCode          string  `json:"error_code"`
	ErrorMessage       string  `json:"error_message"`
	Upstream           struct {
		ResponseID string `json:"response_id"`
	} `json:"upstream"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

const (
	runnerReportStatusQueued  = "queued"
	runnerReportStatusLeased  = "leased"
	runnerReportStatusRunning = "running"
)

type runnerStatusReport struct {
	SourceTaskID string `json:"source_task_id"`
	SubmissionID string `json:"submission_id"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	StartedAt    int64  `json:"started_at"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	Error        struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func isRunnerReportedStatus(status string) bool {
	switch status {
	case runnerReportStatusQueued,
		runnerReportStatusLeased,
		runnerReportStatusRunning,
		model.TaskStatusSucceeded,
		model.TaskStatusFailed,
		model.TaskStatusCanceled:
		return true
	default:
		return false
	}
}

func parseRunnerResult(c echo.Context) (runnerResult, error) {
	contentType := c.Request().Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return parseMultipartRunnerResult(c)
	}

	var req runnerResultMetadata
	if err := c.Bind(&req); err != nil {
		return runnerResult{}, echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if req.Status == model.TaskStatusSucceeded {
		return runnerResult{}, echo.NewHTTPError(http.StatusBadRequest, "multipart/form-data image file is required for succeeded result")
	}
	return runnerResult{
		Status:             req.Status,
		ErrorCode:          firstNonEmpty(req.ErrorCode, req.Error.Code),
		ErrorMessage:       firstNonEmpty(req.ErrorMessage, req.Error.Message),
		UpstreamResponseID: firstNonEmpty(req.UpstreamResponseID, req.Upstream.ResponseID),
	}, nil
}

func parseMultipartRunnerResult(c echo.Context) (runnerResult, error) {
	result := runnerResult{}
	if metadata := strings.TrimSpace(c.FormValue("metadata")); metadata != "" {
		var meta runnerResultMetadata
		if err := json.Unmarshal([]byte(metadata), &meta); err != nil {
			return runnerResult{}, echo.NewHTTPError(http.StatusBadRequest, "invalid metadata")
		}
		result.Status = meta.Status
		result.Width = meta.Width
		result.Height = meta.Height
		result.SizeBytes = meta.SizeBytes
		result.DurationSeconds = meta.DurationSeconds
		result.ErrorCode = firstNonEmpty(meta.ErrorCode, meta.Error.Code)
		result.ErrorMessage = firstNonEmpty(meta.ErrorMessage, meta.Error.Message)
		result.UpstreamResponseID = firstNonEmpty(meta.UpstreamResponseID, meta.Upstream.ResponseID)
	}

	if status := strings.TrimSpace(c.FormValue("status")); status != "" {
		result.Status = status
	}
	if value := strings.TrimSpace(c.FormValue("width")); value != "" {
		result.Width = parseFormInt(value, result.Width)
	}
	if value := strings.TrimSpace(c.FormValue("height")); value != "" {
		result.Height = parseFormInt(value, result.Height)
	}
	if value := strings.TrimSpace(c.FormValue("size_bytes")); value != "" {
		result.SizeBytes = parseFormInt64(value, result.SizeBytes)
	}
	if value := strings.TrimSpace(c.FormValue("duration_seconds")); value != "" {
		result.DurationSeconds = parseFormFloat(value, result.DurationSeconds)
	}
	if value := strings.TrimSpace(c.FormValue("upstream_response_id")); value != "" {
		result.UpstreamResponseID = value
	}
	if value := strings.TrimSpace(c.FormValue("error_code")); value != "" {
		result.ErrorCode = value
	}
	if value := strings.TrimSpace(c.FormValue("error_message")); value != "" {
		result.ErrorMessage = value
	}

	file, err := c.FormFile("image")
	if err == nil {
		result.Image = file
	} else if err != http.ErrMissingFile {
		return runnerResult{}, echo.NewHTTPError(http.StatusBadRequest, "invalid image file")
	}
	return result, nil
}

func parseFormInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseFormInt64(value string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseFormFloat(value string, fallback float64) float64 {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func unixTimestamp(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	t := time.Unix(value, 0)
	return &t
}

func firstNonNilTime(values ...*time.Time) *time.Time {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

func firstNonZero64(a, b int64) int64 {
	if a != 0 {
		return a
	}
	return b
}
