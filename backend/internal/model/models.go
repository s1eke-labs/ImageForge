package model

import "time"

const (
	RunnerStatusOnline  = "online"
	RunnerStatusOffline = "offline"
	RunnerStatusBusy    = "busy"

	TaskStatusPending   = "pending"
	TaskStatusClaimed   = "claimed"
	TaskStatusSucceeded = "succeeded"
	TaskStatusFailed    = "failed"
	TaskStatusCanceled  = "canceled"
)

const (
	RunnerOfflineAfter    = 60 * time.Second
	TaskGenerationTimeout = 30 * time.Minute
)

type User struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"not null" json:"-"`
	TokenVersion int       `gorm:"not null;default:1" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Runner struct {
	ID              string     `gorm:"primaryKey" json:"id"`
	Name            string     `gorm:"not null" json:"name"`
	TokenHash       string     `gorm:"not null" json:"-"`
	TokenPrefix     string     `gorm:"not null;index" json:"token_prefix"`
	Status          string     `gorm:"not null;default:offline" json:"status"`
	Version         string     `json:"version"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Task struct {
	ID                  string     `gorm:"primaryKey" json:"id"`
	UserID              uint       `gorm:"not null;index" json:"user_id"`
	User                User       `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Prompt              string     `gorm:"not null" json:"prompt"`
	Size                string     `gorm:"not null;default:1024x1024" json:"size"`
	Quality             string     `gorm:"not null;default:auto" json:"quality"`
	ReferenceImagePath  string     `json:"reference_image_path,omitempty"`
	ReferenceImagePaths string     `gorm:"type:text" json:"-"`
	Status              string     `gorm:"not null;default:pending;index" json:"status"`
	RunnerID            *string    `gorm:"index" json:"runner_id,omitempty"`
	ResultImagePath     string     `json:"result_image_path,omitempty"`
	ResultWidth         int        `json:"result_width,omitempty"`
	ResultHeight        int        `json:"result_height,omitempty"`
	ResultSizeBytes     int64      `json:"result_size_bytes,omitempty"`
	DurationSeconds     float64    `json:"duration_seconds,omitempty"`
	ErrorCode           string     `json:"error_code,omitempty"`
	ErrorMessage        string     `json:"error_message,omitempty"`
	UpstreamResponseID  string     `json:"upstream_response_id,omitempty"`
	SubmissionID        string     `gorm:"index" json:"submission_id,omitempty"`
	UpstreamStatus      string     `json:"upstream_status,omitempty"`
	UpstreamUpdatedAt   *time.Time `json:"upstream_updated_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	ClaimedAt           *time.Time `json:"claimed_at,omitempty"`
	FinishedAt          *time.Time `json:"finished_at,omitempty"`
}

func (r Runner) EffectiveStatus(now time.Time) string {
	if r.LastHeartbeatAt == nil || now.Sub(*r.LastHeartbeatAt) > RunnerOfflineAfter {
		return RunnerStatusOffline
	}
	if r.Status == RunnerStatusOffline {
		return RunnerStatusOffline
	}
	if r.Status == RunnerStatusBusy {
		return RunnerStatusBusy
	}
	return RunnerStatusOnline
}

func IsTerminalTaskStatus(status string) bool {
	return status == TaskStatusSucceeded || status == TaskStatusFailed || status == TaskStatusCanceled
}
