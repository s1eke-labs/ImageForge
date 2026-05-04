package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

type LoginFailureLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	maxFails int
	failures map[string]loginFailure
}

type loginFailure struct {
	count      int
	windowEnds time.Time
}

func NewLoginFailureLimiter(window time.Duration, maxFails int) *LoginFailureLimiter {
	return &LoginFailureLimiter{
		window:   window,
		maxFails: maxFails,
		failures: map[string]loginFailure{},
	}
}

func (l *LoginFailureLimiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			username, err := readLoginUsername(c)
			if err != nil {
				return err
			}
			key := c.RealIP() + "\x00" + username
			now := time.Now()
			if l.blocked(key, now) {
				return echo.NewHTTPError(http.StatusTooManyRequests, "too many login attempts")
			}

			err = next(c)
			status := c.Response().Status
			if httpErr, ok := err.(*echo.HTTPError); ok {
				status = httpErr.Code
			}
			switch status {
			case http.StatusOK:
				l.clear(key)
			case http.StatusUnauthorized:
				l.recordFailure(key, now)
			}
			return err
		}
	}
}

func (l *LoginFailureLimiter) blocked(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	failure, ok := l.failures[key]
	if !ok || now.After(failure.windowEnds) {
		return false
	}
	return failure.count >= l.maxFails
}

func (l *LoginFailureLimiter) recordFailure(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	failure, ok := l.failures[key]
	if !ok || now.After(failure.windowEnds) {
		l.failures[key] = loginFailure{count: 1, windowEnds: now.Add(l.window)}
		return
	}
	failure.count++
	l.failures[key] = failure
}

func (l *LoginFailureLimiter) clear(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

func readLoginUsername(c echo.Context) (string, error) {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return "", err
	}
	c.Request().Body = io.NopCloser(bytes.NewReader(body))

	var req struct {
		Username string `json:"username"`
	}
	_ = json.Unmarshal(body, &req)
	return req.Username, nil
}
