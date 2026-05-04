package server

import (
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"imageforge/backend/internal/config"
	"imageforge/backend/internal/frontend"
	"imageforge/backend/internal/handler"
	"imageforge/backend/internal/middleware"
	"imageforge/backend/internal/service"

	"github.com/labstack/echo/v4"
	emw "github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Use(emw.Recover())
	e.Use(emw.Logger())
	e.Use(emw.CORS())

	authHandler := handler.AuthHandler{DB: db, JWTSecret: cfg.JWTSecret}
	taskHandler := handler.TaskHandler{DB: db, DataDir: cfg.DataDir}
	runnerHandler := handler.RunnerHandler{DB: db, DataDir: cfg.DataDir}
	fileHandler := handler.FileHandler{DB: db, DataDir: cfg.DataDir}

	api := e.Group("/api")
	loginLimiter := middleware.NewLoginFailureLimiter(time.Minute, 5)
	api.POST("/auth/login", authHandler.Login, loginLimiter.Middleware())

	userAPI := api.Group("", middleware.JWTAuth(db, cfg.JWTSecret))
	userAPI.GET("/tasks", taskHandler.List)
	userAPI.POST("/tasks", taskHandler.Create)
	userAPI.GET("/tasks/:id", taskHandler.Get)
	userAPI.POST("/tasks/:id/retry", taskHandler.Retry)
	userAPI.GET("/runners", runnerHandler.List)
	userAPI.DELETE("/runners/:id", runnerHandler.Delete)
	userAPI.GET("/files/*", fileHandler.UserFile)

	runnerAPI := api.Group("/runner", middleware.RunnerAuth(db))
	runnerAPI.POST("/runners/register", runnerHandler.Register)
	runnerAPI.POST("/runners/:runner_id/heartbeat", runnerHandler.Heartbeat)
	runnerAPI.POST("/runners/:runner_id/tasks/claim", runnerHandler.Claim)
	runnerAPI.POST("/tasks/:task_id/status", runnerHandler.Status)
	runnerAPI.POST("/tasks/:task_id/result", runnerHandler.Result)
	runnerAPI.GET("/files/*", fileHandler.RunnerFile)

	stopJanitor := StartStalledTaskJanitor(db, 30*time.Second, log.Printf)
	e.Server.RegisterOnShutdown(stopJanitor)
	if err := RegisterFrontend(e); err != nil {
		log.Fatal(err)
	}
	return e
}

func StartStalledTaskJanitor(db *gorm.DB, interval time.Duration, logf func(string, ...any)) func() {
	stop := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := service.FailStalledTasks(db, time.Now()); err != nil {
					logf("stalled task cleanup failed: %v", err)
				}
			case <-stop:
				return
			}
		}
	}()
	return func() {
		once.Do(func() {
			close(stop)
		})
	}
}

func RegisterFrontend(e *echo.Echo) error {
	return RegisterFrontendFS(e, frontend.FS)
}

func RegisterFrontendFS(e *echo.Echo, source fs.FS) error {
	static, err := fs.Sub(source, "dist")
	if err != nil {
		return err
	}
	handler := http.FileServer(http.FS(static))
	e.GET("/*", func(c echo.Context) error {
		path := strings.TrimPrefix(c.Request().URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if file, err := static.Open(path); err == nil {
			_ = file.Close()
			handler.ServeHTTP(c.Response(), c.Request())
			return nil
		}
		c.Request().URL.Path = "/"
		index, err := static.Open("index.html")
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "frontend index not found")
		}
		return c.Stream(http.StatusOK, "text/html; charset=utf-8", index)
	})
	return nil
}
