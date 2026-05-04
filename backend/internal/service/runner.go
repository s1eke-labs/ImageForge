package service

import (
	"errors"
	"strings"
	"time"

	"imageforge/backend/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type RunnerCredentials struct {
	RunnerID string
	Name     string
	Token    string
}

func CreateRunner(db *gorm.DB, name string) (RunnerCredentials, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return RunnerCredentials{}, errors.New("name is required")
	}
	runnerID, err := NewID("runner")
	if err != nil {
		return RunnerCredentials{}, err
	}
	token, err := NewRunnerToken()
	if err != nil {
		return RunnerCredentials{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return RunnerCredentials{}, err
	}
	runner := model.Runner{
		ID:          runnerID,
		Name:        name,
		TokenHash:   string(hash),
		TokenPrefix: token[:8],
		Status:      model.RunnerStatusOffline,
		CreatedAt:   time.Now(),
	}
	if err := db.Create(&runner).Error; err != nil {
		return RunnerCredentials{}, err
	}
	return RunnerCredentials{
		RunnerID: runner.ID,
		Name:     runner.Name,
		Token:    token,
	}, nil
}
