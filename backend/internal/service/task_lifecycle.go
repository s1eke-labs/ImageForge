package service

import (
	"time"

	"imageforge/backend/internal/model"

	"gorm.io/gorm"
)

const (
	runnerOfflineErrorCode = "RUNNER_OFFLINE"
	taskTimeoutErrorCode   = "TASK_TIMEOUT"
)

func FailStalledTasks(db *gorm.DB, now time.Time) error {
	if err := FailTasksForOfflineRunners(db, now); err != nil {
		return err
	}
	return FailTimedOutTasks(db, now)
}

func FailTasksForOfflineRunners(db *gorm.DB, now time.Time) error {
	cutoff := now.Add(-model.RunnerOfflineAfter)
	offlineRunners := db.Model(&model.Runner{}).
		Select("id").
		Where("status = ? OR last_heartbeat_at IS NULL OR last_heartbeat_at < ?", model.RunnerStatusOffline, cutoff)

	return db.Model(&model.Task{}).
		Where("status = ? AND runner_id IN (?)", model.TaskStatusClaimed, offlineRunners).
		Updates(map[string]any{
			"status":        model.TaskStatusFailed,
			"error_code":    runnerOfflineErrorCode,
			"error_message": "Runner went offline before reporting a result",
			"finished_at":   &now,
		}).Error
}

func FailTimedOutTasks(db *gorm.DB, now time.Time) error {
	cutoff := now.Add(-model.TaskGenerationTimeout)
	return db.Model(&model.Task{}).
		Where("status = ? AND ((claimed_at IS NOT NULL AND claimed_at < ?) OR (claimed_at IS NULL AND created_at < ?))", model.TaskStatusClaimed, cutoff, cutoff).
		Updates(map[string]any{
			"status":        model.TaskStatusFailed,
			"error_code":    taskTimeoutErrorCode,
			"error_message": "Task exceeded the 30 minute generation timeout",
			"finished_at":   &now,
		}).Error
}
