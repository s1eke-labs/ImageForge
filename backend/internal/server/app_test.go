package server_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"imageforge/backend/internal/config"
	"imageforge/backend/internal/database"
	"imageforge/backend/internal/middleware"
	"imageforge/backend/internal/model"
	"imageforge/backend/internal/server"
	"imageforge/backend/internal/service"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestUserAndRunnerFlow(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	db, err := database.Open(filepath.Join(dataDir, "imageforge.db"))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("test123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "admin", PasswordHash: string(hash)}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	e := server.New(config.Config{DataDir: dataDir, JWTSecret: "test-secret"}, db)

	login := postJSON(t, e, "/api/auth/login", map[string]any{"username": "admin", "password": "test123"}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}
	var loginResp struct {
		Token string `json:"token"`
	}
	decode(t, login.Body.Bytes(), &loginResp)
	userBearer := "Bearer " + loginResp.Token

	taskResp := postMultipartTaskWithReference(t, e, userBearer)
	if taskResp.Code != http.StatusCreated {
		t.Fatalf("create task status = %d, body = %s", taskResp.Code, taskResp.Body.String())
	}
	var createdTask struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	decode(t, taskResp.Body.Bytes(), &createdTask)
	if createdTask.Status != model.TaskStatusPending {
		t.Fatalf("created task status = %s", createdTask.Status)
	}

	runner, err := service.CreateRunner(db, "runner-a")
	if err != nil {
		t.Fatal(err)
	}
	runnerBearer := "Bearer " + runner.Token
	if runner.RunnerID == "" || runner.Token == "" {
		t.Fatalf("runner credentials incomplete: %#v", runner)
	}

	register := postJSON(t, e, "/api/runner/runners/register", map[string]any{"name": "runner-a", "version": "0.1.0"}, runnerBearer)
	if register.Code != http.StatusOK {
		t.Fatalf("runner register status = %d, body = %s", register.Code, register.Body.String())
	}
	heartbeat := postJSON(t, e, "/api/runner/runners/"+runner.RunnerID+"/heartbeat", map[string]any{"status": "online", "version": "0.1.0"}, runnerBearer)
	if heartbeat.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d", heartbeat.Code)
	}

	claim := postJSON(t, e, "/api/runner/runners/"+runner.RunnerID+"/tasks/claim", map[string]any{"limit": 1}, runnerBearer)
	if claim.Code != http.StatusOK {
		t.Fatalf("claim status = %d, body = %s", claim.Code, claim.Body.String())
	}
	var claimResp struct {
		Tasks []struct {
			ID             string `json:"id"`
			SourceTaskID   string `json:"source_task_id"`
			IdempotencyKey string `json:"idempotency_key"`
			Payload        struct {
				Prompt          string `json:"prompt"`
				Size            string `json:"size"`
				Quality         string `json:"quality"`
				ReferenceImages []struct {
					B64JSON  string `json:"b64_json"`
					FileName string `json:"file_name"`
					MIMEType string `json:"mime_type"`
					URL      string `json:"url"`
				} `json:"reference_images"`
			} `json:"payload"`
		} `json:"tasks"`
	}
	decode(t, claim.Body.Bytes(), &claimResp)
	if len(claimResp.Tasks) != 1 || claimResp.Tasks[0].ID != createdTask.ID || claimResp.Tasks[0].IdempotencyKey != createdTask.ID {
		t.Fatalf("unexpected claim response: %#v", claimResp)
	}
	references := claimResp.Tasks[0].Payload.ReferenceImages
	if len(references) != 1 || references[0].B64JSON != tinyPNGBase64 || references[0].FileName != "ref-1.png" || references[0].MIMEType != "image/png" || references[0].URL != "" {
		t.Fatalf("runner reference image should include b64_json, file_name, and mime_type only: %#v", references)
	}
	var rawClaim map[string][]map[string]any
	decode(t, claim.Body.Bytes(), &rawClaim)
	if _, ok := rawClaim["tasks"][0]["payload"].(map[string]any)["response_format"]; ok {
		t.Fatalf("claim payload unexpectedly includes response_format: %s", claim.Body.String())
	}

	result := postMultipartResult(t, e, "/api/runner/tasks/"+createdTask.ID+"/result", map[string]string{
		"source_task_id":       createdTask.ID,
		"status":               "succeeded",
		"width":                "1",
		"height":               "1",
		"duration_seconds":     "0.25",
		"upstream_response_id": "resp_test",
	}, tinyPNGBytes(t), runnerBearer)
	if result.Code != http.StatusOK {
		t.Fatalf("result status = %d, body = %s", result.Code, result.Body.String())
	}
	var resultResp struct {
		Status          string `json:"status"`
		ResultImagePath string `json:"result_image_path"`
	}
	decode(t, result.Body.Bytes(), &resultResp)
	if resultResp.Status != model.TaskStatusSucceeded || resultResp.ResultImagePath == "" {
		t.Fatalf("unexpected result response: %#v", resultResp)
	}
	assertTaskImagePath(t, resultResp.ResultImagePath, createdTask.ID)

	file := get(t, e, "/api/files/"+resultResp.ResultImagePath, userBearer)
	if file.Code != http.StatusOK {
		t.Fatalf("file download status = %d", file.Code)
	}
}

func TestRunnerResultRequiresImageFile(t *testing.T) {
	t.Parallel()
	ctx := setupClaimedTask(t)

	result := postMultipartResult(t, ctx.Handler, "/api/runner/tasks/"+ctx.TaskID+"/result", map[string]string{
		"source_task_id": ctx.TaskID,
		"status":         "succeeded",
		"width":          "1",
		"height":         "1",
	}, nil, ctx.RunnerBearer)
	if result.Code != http.StatusBadRequest {
		t.Fatalf("missing-image result status = %d, body = %s", result.Code, result.Body.String())
	}

	task := get(t, ctx.Handler, "/api/tasks/"+ctx.TaskID, ctx.UserBearer)
	if task.Code != http.StatusOK {
		t.Fatalf("task get status = %d, body = %s", task.Code, task.Body.String())
	}
	var taskResp struct {
		Status string `json:"status"`
	}
	decode(t, task.Body.Bytes(), &taskResp)
	if taskResp.Status != model.TaskStatusClaimed {
		t.Fatalf("task status after missing-image result = %s", taskResp.Status)
	}
}

func TestLoginRateLimitByIPAndUsername(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	db, err := database.Open(filepath.Join(dataDir, "imageforge.db"))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("test123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{Username: "admin", PasswordHash: string(hash), TokenVersion: 1}).Error; err != nil {
		t.Fatal(err)
	}
	e := server.New(config.Config{DataDir: dataDir, JWTSecret: "test-secret"}, db)

	for i := 0; i < 4; i++ {
		resp := postJSONFromIP(t, e, "/api/auth/login", map[string]any{"username": "admin", "password": "wrong"}, "", "203.0.113.10")
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("wrong password attempt %d status = %d, body = %s", i+1, resp.Code, resp.Body.String())
		}
	}
	success := postJSONFromIP(t, e, "/api/auth/login", map[string]any{"username": "admin", "password": "test123"}, "", "203.0.113.10")
	if success.Code != http.StatusOK {
		t.Fatalf("successful login status = %d, body = %s", success.Code, success.Body.String())
	}
	for i := 0; i < 5; i++ {
		resp := postJSONFromIP(t, e, "/api/auth/login", map[string]any{"username": "admin", "password": "wrong"}, "", "203.0.113.10")
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("post-reset wrong password attempt %d status = %d, body = %s", i+1, resp.Code, resp.Body.String())
		}
	}
	limited := postJSONFromIP(t, e, "/api/auth/login", map[string]any{"username": "admin", "password": "wrong"}, "", "203.0.113.10")
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("sixth wrong password status = %d, body = %s", limited.Code, limited.Body.String())
	}
	otherUser := postJSONFromIP(t, e, "/api/auth/login", map[string]any{"username": "someone-else", "password": "wrong"}, "", "203.0.113.10")
	if otherUser.Code != http.StatusUnauthorized {
		t.Fatalf("other username should use a separate counter, status = %d, body = %s", otherUser.Code, otherUser.Body.String())
	}
	otherIP := postJSONFromIP(t, e, "/api/auth/login", map[string]any{"username": "admin", "password": "wrong"}, "", "203.0.113.11")
	if otherIP.Code != http.StatusUnauthorized {
		t.Fatalf("other IP should use a separate counter, status = %d, body = %s", otherIP.Code, otherIP.Body.String())
	}
}

func TestJWTTokenExpiresAfterTwoHours(t *testing.T) {
	t.Parallel()
	e, _, _ := setupAuthenticatedUser(t)

	login := postJSON(t, e, "/api/auth/login", map[string]any{"username": "admin", "password": "test123"}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}
	var loginResp struct {
		Token string `json:"token"`
	}
	decode(t, login.Body.Bytes(), &loginResp)

	claims := new(middleware.JWTClaims)
	token, err := jwt.ParseWithClaims(loginResp.Token, claims, func(token *jwt.Token) (any, error) {
		return []byte("test-secret"), nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("parse token: valid=%v err=%v", token.Valid, err)
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil {
		t.Fatalf("token missing exp or iat: %#v", claims)
	}
	if got := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time); got != 2*time.Hour {
		t.Fatalf("token lifetime = %s, want 2h", got)
	}
}

func TestJWTRejectsMissingUserAndExpiredToken(t *testing.T) {
	t.Parallel()
	e, _, _ := setupAuthenticatedUser(t)

	missingUserToken, err := middleware.SignJWT(model.User{ID: 999, Username: "missing", TokenVersion: 1}, "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	missingUser := get(t, e, "/api/tasks", "Bearer "+missingUserToken)
	if missingUser.Code != http.StatusUnauthorized {
		t.Fatalf("missing-user token status = %d, body = %s", missingUser.Code, missingUser.Body.String())
	}

	now := time.Now()
	expiredClaims := middleware.JWTClaims{
		UserID:       1,
		Username:     "admin",
		TokenVersion: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
		},
	}
	expiredToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	expired := get(t, e, "/api/tasks", "Bearer "+expiredToken)
	if expired.Code != http.StatusUnauthorized {
		t.Fatalf("expired token status = %d, body = %s", expired.Code, expired.Body.String())
	}
}

func TestRunnerStatusReportUpdatesClaimedTask(t *testing.T) {
	t.Parallel()
	ctx := setupClaimedTask(t)
	startedAt := time.Now().Add(-2 * time.Second).Unix()
	updatedAt := time.Now().Unix()

	status := postJSON(t, ctx.Handler, "/api/runner/tasks/"+ctx.TaskID+"/status", map[string]any{
		"source_task_id": ctx.TaskID,
		"submission_id":  "sub_test",
		"status":         "running",
		"updated_at":     updatedAt,
		"started_at":     startedAt,
	}, ctx.RunnerBearer)
	if status.Code != http.StatusOK {
		t.Fatalf("running status = %d, body = %s", status.Code, status.Body.String())
	}
	var statusResp struct {
		Status            string `json:"status"`
		SubmissionID      string `json:"submission_id"`
		UpstreamStatus    string `json:"upstream_status"`
		UpstreamUpdatedAt string `json:"upstream_updated_at"`
	}
	decode(t, status.Body.Bytes(), &statusResp)
	if statusResp.Status != model.TaskStatusClaimed || statusResp.SubmissionID != "sub_test" || statusResp.UpstreamStatus != "running" || statusResp.UpstreamUpdatedAt == "" {
		t.Fatalf("unexpected running status response: %#v", statusResp)
	}

	stale := postJSON(t, ctx.Handler, "/api/runner/tasks/"+ctx.TaskID+"/status", map[string]any{
		"source_task_id": ctx.TaskID,
		"submission_id":  "sub_test",
		"status":         "queued",
		"updated_at":     updatedAt - 30,
	}, ctx.RunnerBearer)
	if stale.Code != http.StatusOK {
		t.Fatalf("stale queued status = %d, body = %s", stale.Code, stale.Body.String())
	}
	var staleResp struct {
		Status         string `json:"status"`
		UpstreamStatus string `json:"upstream_status"`
	}
	decode(t, stale.Body.Bytes(), &staleResp)
	if staleResp.Status != model.TaskStatusClaimed || staleResp.UpstreamStatus != "running" {
		t.Fatalf("stale status should not regress task: %#v", staleResp)
	}
}

func TestRunnerStatusReportFailedAndCanceledAreTerminal(t *testing.T) {
	t.Parallel()
	ctx := setupClaimedTask(t)

	failed := postJSON(t, ctx.Handler, "/api/runner/tasks/"+ctx.TaskID+"/status", map[string]any{
		"source_task_id": ctx.TaskID,
		"submission_id":  "sub_failed",
		"status":         "failed",
		"updated_at":     time.Now().Unix(),
		"error": map[string]any{
			"code":    "IMAGE_UPSTREAM_ERROR",
			"message": "Image generation failed",
		},
	}, ctx.RunnerBearer)
	if failed.Code != http.StatusOK {
		t.Fatalf("failed status = %d, body = %s", failed.Code, failed.Body.String())
	}
	var failedResp struct {
		Status       string `json:"status"`
		ErrorCode    string `json:"error_code"`
		ErrorMessage string `json:"error_message"`
		FinishedAt   string `json:"finished_at"`
	}
	decode(t, failed.Body.Bytes(), &failedResp)
	if failedResp.Status != model.TaskStatusFailed || failedResp.ErrorCode != "IMAGE_UPSTREAM_ERROR" || failedResp.ErrorMessage == "" || failedResp.FinishedAt == "" {
		t.Fatalf("unexpected failed status response: %#v", failedResp)
	}

	canceledCtx := setupClaimedTask(t)
	canceled := postJSON(t, canceledCtx.Handler, "/api/runner/tasks/"+canceledCtx.TaskID+"/status", map[string]any{
		"source_task_id": canceledCtx.TaskID,
		"submission_id":  "sub_canceled",
		"status":         "canceled",
		"updated_at":     time.Now().Unix(),
		"error": map[string]any{
			"code":    "USER_CANCELED",
			"message": "Canceled upstream",
		},
	}, canceledCtx.RunnerBearer)
	if canceled.Code != http.StatusOK {
		t.Fatalf("canceled status = %d, body = %s", canceled.Code, canceled.Body.String())
	}
	var canceledResp struct {
		Status       string `json:"status"`
		ErrorCode    string `json:"error_code"`
		ErrorMessage string `json:"error_message"`
		FinishedAt   string `json:"finished_at"`
	}
	decode(t, canceled.Body.Bytes(), &canceledResp)
	if canceledResp.Status != model.TaskStatusCanceled || canceledResp.ErrorCode != "USER_CANCELED" || canceledResp.ErrorMessage == "" || canceledResp.FinishedAt == "" {
		t.Fatalf("unexpected canceled status response: %#v", canceledResp)
	}
}

func TestRunnerStatusReportSucceededDoesNotReplaceResultUpload(t *testing.T) {
	t.Parallel()
	ctx := setupClaimedTask(t)

	status := postJSON(t, ctx.Handler, "/api/runner/tasks/"+ctx.TaskID+"/status", map[string]any{
		"source_task_id": ctx.TaskID,
		"submission_id":  "sub_success",
		"status":         "succeeded",
		"updated_at":     time.Now().Unix(),
	}, ctx.RunnerBearer)
	if status.Code != http.StatusOK {
		t.Fatalf("succeeded status = %d, body = %s", status.Code, status.Body.String())
	}
	var statusResp struct {
		Status          string `json:"status"`
		UpstreamStatus  string `json:"upstream_status"`
		ResultImagePath string `json:"result_image_path"`
	}
	decode(t, status.Body.Bytes(), &statusResp)
	if statusResp.Status != model.TaskStatusClaimed || statusResp.UpstreamStatus != model.TaskStatusSucceeded || statusResp.ResultImagePath != "" {
		t.Fatalf("succeeded status should not complete without result image: %#v", statusResp)
	}

	result := postMultipartResult(t, ctx.Handler, "/api/runner/tasks/"+ctx.TaskID+"/result", map[string]string{
		"source_task_id": ctx.TaskID,
		"status":         "succeeded",
		"width":          "1",
		"height":         "1",
	}, tinyPNGBytes(t), ctx.RunnerBearer)
	if result.Code != http.StatusOK {
		t.Fatalf("result after succeeded status = %d, body = %s", result.Code, result.Body.String())
	}
	var resultResp struct {
		Status          string `json:"status"`
		ResultImagePath string `json:"result_image_path"`
	}
	decode(t, result.Body.Bytes(), &resultResp)
	if resultResp.Status != model.TaskStatusSucceeded || resultResp.ResultImagePath == "" {
		t.Fatalf("unexpected result response after status report: %#v", resultResp)
	}
}

func TestRunnerStatusReportDoesNotOverwriteSucceededResult(t *testing.T) {
	t.Parallel()
	ctx := setupClaimedTask(t)

	result := postMultipartResult(t, ctx.Handler, "/api/runner/tasks/"+ctx.TaskID+"/result", map[string]string{
		"source_task_id": ctx.TaskID,
		"status":         "succeeded",
		"width":          "1",
		"height":         "1",
	}, tinyPNGBytes(t), ctx.RunnerBearer)
	if result.Code != http.StatusOK {
		t.Fatalf("result status = %d, body = %s", result.Code, result.Body.String())
	}
	var resultResp struct {
		Status          string `json:"status"`
		ResultImagePath string `json:"result_image_path"`
		ResultWidth     int    `json:"result_width"`
		ResultHeight    int    `json:"result_height"`
		FinishedAt      string `json:"finished_at"`
	}
	decode(t, result.Body.Bytes(), &resultResp)
	if resultResp.Status != model.TaskStatusSucceeded || resultResp.ResultImagePath == "" || resultResp.FinishedAt == "" {
		t.Fatalf("unexpected result response: %#v", resultResp)
	}

	reports := []string{"running", "succeeded", "failed"}
	for _, status := range reports {
		resp := postJSON(t, ctx.Handler, "/api/runner/tasks/"+ctx.TaskID+"/status", map[string]any{
			"source_task_id": ctx.TaskID,
			"submission_id":  "sub_" + status,
			"status":         status,
			"updated_at":     time.Now().Unix(),
			"error": map[string]any{
				"code":    "SHOULD_NOT_WRITE",
				"message": "should not overwrite result",
			},
		}, ctx.RunnerBearer)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s status report = %d, body = %s", status, resp.Code, resp.Body.String())
		}
		var taskResp struct {
			Status          string `json:"status"`
			ResultImagePath string `json:"result_image_path"`
			ResultWidth     int    `json:"result_width"`
			ResultHeight    int    `json:"result_height"`
			ErrorCode       string `json:"error_code"`
			FinishedAt      string `json:"finished_at"`
		}
		decode(t, resp.Body.Bytes(), &taskResp)
		if taskResp.Status != model.TaskStatusSucceeded ||
			taskResp.ResultImagePath != resultResp.ResultImagePath ||
			taskResp.ResultWidth != resultResp.ResultWidth ||
			taskResp.ResultHeight != resultResp.ResultHeight ||
			taskResp.ErrorCode != "" ||
			taskResp.FinishedAt != resultResp.FinishedAt {
			t.Fatalf("%s status report overwrote result fields: %#v", status, taskResp)
		}
	}
}

func TestRunnerStatusUpdateFailureDoesNotReturnOK(t *testing.T) {
	t.Parallel()
	ctx := setupClaimedTask(t)
	if err := ctx.DB.Exec(`
		CREATE TRIGGER fail_runner_status_update
		BEFORE UPDATE OF status ON runners
		BEGIN
			SELECT RAISE(ABORT, 'runner status update failed');
		END;
	`).Error; err != nil {
		t.Fatal(err)
	}

	status := postJSON(t, ctx.Handler, "/api/runner/tasks/"+ctx.TaskID+"/status", map[string]any{
		"source_task_id": ctx.TaskID,
		"status":         "running",
		"updated_at":     time.Now().Unix(),
	}, ctx.RunnerBearer)
	if status.Code == http.StatusOK {
		t.Fatalf("runner status update failure returned 200, body = %s", status.Body.String())
	}
}

func TestUserReadDoesNotFailClaimedTaskWhenRunnerGoesOffline(t *testing.T) {
	t.Parallel()
	ctx := setupClaimedTask(t)
	staleHeartbeat := time.Now().Add(-2 * model.RunnerOfflineAfter)
	if err := ctx.DB.Model(&model.Runner{}).Where("id = ?", ctx.RunnerID).Update("last_heartbeat_at", &staleHeartbeat).Error; err != nil {
		t.Fatal(err)
	}

	task := get(t, ctx.Handler, "/api/tasks/"+ctx.TaskID, ctx.UserBearer)
	if task.Code != http.StatusOK {
		t.Fatalf("task get status = %d, body = %s", task.Code, task.Body.String())
	}
	var taskResp struct {
		Status       string `json:"status"`
		ErrorCode    string `json:"error_code"`
		ErrorMessage string `json:"error_message"`
		FinishedAt   string `json:"finished_at"`
	}
	decode(t, task.Body.Bytes(), &taskResp)
	if taskResp.Status != model.TaskStatusClaimed || taskResp.ErrorCode != "" || taskResp.FinishedAt != "" {
		t.Fatalf("user read should not fail stalled task: %#v", taskResp)
	}
}

func TestClaimedTaskFailsWhenRunnerReportsOffline(t *testing.T) {
	t.Parallel()
	ctx := setupClaimedTask(t)

	offline := postJSON(t, ctx.Handler, "/api/runner/runners/"+ctx.RunnerID+"/heartbeat", map[string]any{
		"status":  "offline",
		"version": "0.1.0",
	}, ctx.RunnerBearer)
	if offline.Code != http.StatusOK {
		t.Fatalf("offline heartbeat status = %d, body = %s", offline.Code, offline.Body.String())
	}

	task := get(t, ctx.Handler, "/api/tasks/"+ctx.TaskID, ctx.UserBearer)
	if task.Code != http.StatusOK {
		t.Fatalf("task get status = %d, body = %s", task.Code, task.Body.String())
	}
	var taskResp struct {
		Status       string `json:"status"`
		ErrorCode    string `json:"error_code"`
		ErrorMessage string `json:"error_message"`
		FinishedAt   string `json:"finished_at"`
	}
	decode(t, task.Body.Bytes(), &taskResp)
	if taskResp.Status != model.TaskStatusFailed {
		t.Fatalf("task status after offline heartbeat = %s", taskResp.Status)
	}
	if taskResp.ErrorCode != "RUNNER_OFFLINE" || taskResp.ErrorMessage == "" || taskResp.FinishedAt == "" {
		t.Fatalf("unexpected offline failure response: %#v", taskResp)
	}

	runners := get(t, ctx.Handler, "/api/runners", ctx.UserBearer)
	if runners.Code != http.StatusOK {
		t.Fatalf("runner list status = %d, body = %s", runners.Code, runners.Body.String())
	}
	var runnersResp struct {
		Runners []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"runners"`
	}
	decode(t, runners.Body.Bytes(), &runnersResp)
	if len(runnersResp.Runners) != 1 || runnersResp.Runners[0].Status != model.RunnerStatusOffline {
		t.Fatalf("unexpected runner status response: %#v", runnersResp)
	}
}

func TestClaimedTaskFailsAfterGenerationTimeout(t *testing.T) {
	t.Parallel()
	ctx := setupClaimedTask(t)
	staleClaim := time.Now().Add(-2 * model.TaskGenerationTimeout)
	if err := ctx.DB.Model(&model.Task{}).Where("id = ?", ctx.TaskID).Update("claimed_at", &staleClaim).Error; err != nil {
		t.Fatal(err)
	}

	stop := server.StartStalledTaskJanitor(ctx.DB, 10*time.Millisecond, t.Logf)
	defer stop()
	waitForTaskStatus(t, ctx.DB, ctx.TaskID, model.TaskStatusFailed)

	task := get(t, ctx.Handler, "/api/tasks/"+ctx.TaskID, ctx.UserBearer)
	if task.Code != http.StatusOK {
		t.Fatalf("task get status = %d, body = %s", task.Code, task.Body.String())
	}
	var taskResp struct {
		Status       string `json:"status"`
		ErrorCode    string `json:"error_code"`
		ErrorMessage string `json:"error_message"`
		FinishedAt   string `json:"finished_at"`
	}
	decode(t, task.Body.Bytes(), &taskResp)
	if taskResp.Status != model.TaskStatusFailed || taskResp.ErrorCode != "TASK_TIMEOUT" || taskResp.ErrorMessage == "" || taskResp.FinishedAt == "" {
		t.Fatalf("unexpected timeout failure response: %#v", taskResp)
	}
}

func TestTaskCreateAcceptsFourReferenceImages(t *testing.T) {
	t.Parallel()
	e, db, userBearer := setupAuthenticatedUser(t)

	taskResp := postMultipartTaskWithReferenceImages(t, e, userBearer, 4)
	if taskResp.Code != http.StatusCreated {
		t.Fatalf("create task with references status = %d, body = %s", taskResp.Code, taskResp.Body.String())
	}
	var createdTask struct {
		ID                  string   `json:"id"`
		ReferenceImagePath  string   `json:"reference_image_path"`
		ReferenceThumbPath  string   `json:"reference_thumb_path"`
		ReferenceImagePaths []string `json:"reference_image_paths"`
		ReferenceThumbPaths []string `json:"reference_thumb_paths"`
	}
	decode(t, taskResp.Body.Bytes(), &createdTask)
	if len(createdTask.ReferenceImagePaths) != 4 || len(createdTask.ReferenceThumbPaths) != 4 {
		t.Fatalf("unexpected reference paths: %#v", createdTask)
	}
	if createdTask.ReferenceImagePath != createdTask.ReferenceImagePaths[0] || createdTask.ReferenceThumbPath != createdTask.ReferenceThumbPaths[0] {
		t.Fatalf("legacy reference fields should mirror first reference: %#v", createdTask)
	}
	if filepath.Base(createdTask.ReferenceImagePaths[0]) != "ref-1.png" || filepath.Base(createdTask.ReferenceImagePaths[3]) != "ref-4.png" {
		t.Fatalf("unexpected ordered reference names: %#v", createdTask.ReferenceImagePaths)
	}
	for _, path := range createdTask.ReferenceImagePaths {
		assertTaskImagePath(t, path, createdTask.ID)
	}

	runner, err := service.CreateRunner(db, "runner-a")
	if err != nil {
		t.Fatal(err)
	}
	runnerBearer := "Bearer " + runner.Token
	register := postJSON(t, e, "/api/runner/runners/register", map[string]any{"name": "runner-a", "version": "0.1.0"}, runnerBearer)
	if register.Code != http.StatusOK {
		t.Fatalf("runner register status = %d, body = %s", register.Code, register.Body.String())
	}
	claim := postJSON(t, e, "/api/runner/runners/"+runner.RunnerID+"/tasks/claim", map[string]any{"limit": 1}, runnerBearer)
	if claim.Code != http.StatusOK {
		t.Fatalf("claim status = %d, body = %s", claim.Code, claim.Body.String())
	}
	var claimResp struct {
		Tasks []struct {
			Payload struct {
				ReferenceImages []struct {
					B64JSON  string `json:"b64_json"`
					FileName string `json:"file_name"`
					MIMEType string `json:"mime_type"`
				} `json:"reference_images"`
			} `json:"payload"`
		} `json:"tasks"`
	}
	decode(t, claim.Body.Bytes(), &claimResp)
	references := claimResp.Tasks[0].Payload.ReferenceImages
	if len(references) != 4 || references[0].FileName != "ref-1.png" || references[3].FileName != "ref-4.png" {
		t.Fatalf("unexpected runner references: %#v", references)
	}
	for _, reference := range references {
		if reference.B64JSON != tinyPNGBase64 || reference.MIMEType != "image/png" {
			t.Fatalf("unexpected reference payload: %#v", reference)
		}
	}
}

func TestTaskCreateRejectsTooManyReferenceImages(t *testing.T) {
	t.Parallel()
	e, _, userBearer := setupAuthenticatedUser(t)

	taskResp := postMultipartTaskWithReferenceImages(t, e, userBearer, 5)
	if taskResp.Code != http.StatusBadRequest || !bytes.Contains(taskResp.Body.Bytes(), []byte("up to 4 reference images")) {
		t.Fatalf("create task with too many references status = %d, body = %s", taskResp.Code, taskResp.Body.String())
	}
}

func TestTaskCreateRejectsOversizedReferenceImage(t *testing.T) {
	t.Parallel()
	e, _, userBearer := setupAuthenticatedUser(t)

	taskResp := postMultipartTaskWithReferencePayloads(t, e, userBearer, [][]byte{bytes.Repeat([]byte{0}, service.MaxReferenceImageBytes+1)}, []string{"large.png"})
	if taskResp.Code != http.StatusBadRequest || !bytes.Contains(taskResp.Body.Bytes(), []byte("reference image exceeds 10MB")) {
		t.Fatalf("create task with oversized reference status = %d, body = %s", taskResp.Code, taskResp.Body.String())
	}
}

func TestTaskCreateRejectsUnsupportedReferenceImage(t *testing.T) {
	t.Parallel()
	e, _, userBearer := setupAuthenticatedUser(t)

	taskResp := postMultipartTaskWithReferencePayloads(t, e, userBearer, [][]byte{[]byte("not an image")}, []string{"broken.png"})
	if taskResp.Code != http.StatusBadRequest || !bytes.Contains(taskResp.Body.Bytes(), []byte("reference image is not a supported image")) {
		t.Fatalf("create task with unsupported reference status = %d, body = %s", taskResp.Code, taskResp.Body.String())
	}
}

func TestFailedTaskRetryCopiesReferenceImages(t *testing.T) {
	t.Parallel()
	e, db, userBearer := setupAuthenticatedUser(t)

	taskResp := postMultipartTaskWithReferenceImages(t, e, userBearer, 2)
	if taskResp.Code != http.StatusCreated {
		t.Fatalf("create task with references status = %d, body = %s", taskResp.Code, taskResp.Body.String())
	}
	var createdTask struct {
		ID string `json:"id"`
	}
	decode(t, taskResp.Body.Bytes(), &createdTask)
	if err := db.Model(&model.Task{}).Where("id = ?", createdTask.ID).Update("status", model.TaskStatusFailed).Error; err != nil {
		t.Fatal(err)
	}

	retry := postJSON(t, e, "/api/tasks/"+createdTask.ID+"/retry", map[string]any{}, userBearer)
	if retry.Code != http.StatusCreated {
		t.Fatalf("retry status = %d, body = %s", retry.Code, retry.Body.String())
	}
	var retryResp struct {
		ID                  string   `json:"id"`
		ReferenceImagePaths []string `json:"reference_image_paths"`
	}
	decode(t, retry.Body.Bytes(), &retryResp)
	if retryResp.ID == "" || retryResp.ID == createdTask.ID || len(retryResp.ReferenceImagePaths) != 2 {
		t.Fatalf("unexpected retry references: %#v", retryResp)
	}
	if filepath.Base(retryResp.ReferenceImagePaths[0]) != "ref-1.png" || filepath.Base(retryResp.ReferenceImagePaths[1]) != "ref-2.png" {
		t.Fatalf("unexpected retry reference names: %#v", retryResp.ReferenceImagePaths)
	}
}

func TestFailedTaskCanBeRetried(t *testing.T) {
	t.Parallel()
	ctx := setupClaimedTask(t)

	failed := postMultipartResult(t, ctx.Handler, "/api/runner/tasks/"+ctx.TaskID+"/result", map[string]string{
		"source_task_id": ctx.TaskID,
		"status":         "failed",
		"error_code":     "UPSTREAM_ERROR",
		"error_message":  "upstream failed",
	}, nil, ctx.RunnerBearer)
	if failed.Code != http.StatusOK {
		t.Fatalf("failed result status = %d, body = %s", failed.Code, failed.Body.String())
	}

	retry := postJSON(t, ctx.Handler, "/api/tasks/"+ctx.TaskID+"/retry", map[string]any{}, ctx.UserBearer)
	if retry.Code != http.StatusCreated {
		t.Fatalf("retry status = %d, body = %s", retry.Code, retry.Body.String())
	}
	var retryResp struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Prompt  string `json:"prompt"`
		Size    string `json:"size"`
		Quality string `json:"quality"`
	}
	decode(t, retry.Body.Bytes(), &retryResp)
	if retryResp.ID == "" || retryResp.ID == ctx.TaskID {
		t.Fatalf("unexpected retry id: %#v", retryResp)
	}
	if retryResp.Status != model.TaskStatusPending || retryResp.Prompt == "" || retryResp.Size != "1024x1024" || retryResp.Quality != "auto" {
		t.Fatalf("unexpected retry response: %#v", retryResp)
	}

	original := get(t, ctx.Handler, "/api/tasks/"+ctx.TaskID, ctx.UserBearer)
	if original.Code != http.StatusNotFound {
		t.Fatalf("original task after retry status = %d, body = %s", original.Code, original.Body.String())
	}

	list := get(t, ctx.Handler, "/api/tasks", ctx.UserBearer)
	if list.Code != http.StatusOK {
		t.Fatalf("task list status = %d, body = %s", list.Code, list.Body.String())
	}
	var listResp struct {
		Tasks []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"tasks"`
	}
	decode(t, list.Body.Bytes(), &listResp)
	for _, task := range listResp.Tasks {
		if task.ID == ctx.TaskID {
			t.Fatalf("original failed task still appears in task list: %#v", listResp)
		}
	}
}

func TestTaskStatusesReturnsCompactUserOwnedUpdates(t *testing.T) {
	t.Parallel()
	e, db, userBearer := setupAuthenticatedUser(t)

	taskResp := postMultipartTask(t, e, userBearer)
	if taskResp.Code != http.StatusCreated {
		t.Fatalf("create task status = %d, body = %s", taskResp.Code, taskResp.Body.String())
	}
	var createdTask struct {
		ID string `json:"id"`
	}
	decode(t, taskResp.Body.Bytes(), &createdTask)

	finishedAt := time.Now().UTC().Truncate(time.Second)
	updates := map[string]any{
		"status":            model.TaskStatusSucceeded,
		"result_image_path": "images/2026/05/03/" + createdTask.ID + "/output/result.png",
		"result_width":      1024,
		"result_height":     1024,
		"duration_seconds":  1.25,
		"upstream_status":   "completed",
		"finished_at":       &finishedAt,
		"error_code":        "",
		"error_message":     "",
	}
	if err := db.Model(&model.Task{}).Where("id = ?", createdTask.ID).Updates(updates).Error; err != nil {
		t.Fatal(err)
	}

	otherUser := model.User{Username: "other", PasswordHash: "hash"}
	if err := db.Create(&otherUser).Error; err != nil {
		t.Fatal(err)
	}
	otherTask := model.Task{ID: "task_other_status", UserID: otherUser.ID, Prompt: "hidden", Size: "1024x1024", Quality: "auto", Status: model.TaskStatusPending}
	if err := db.Create(&otherTask).Error; err != nil {
		t.Fatal(err)
	}

	statuses := get(t, e, "/api/tasks/statuses?ids=missing,"+createdTask.ID+","+otherTask.ID+","+createdTask.ID, userBearer)
	if statuses.Code != http.StatusOK {
		t.Fatalf("statuses status = %d, body = %s", statuses.Code, statuses.Body.String())
	}
	var statusResp struct {
		Tasks []map[string]any `json:"tasks"`
	}
	decode(t, statuses.Body.Bytes(), &statusResp)
	if len(statusResp.Tasks) != 1 {
		t.Fatalf("expected one user-owned status update, got %#v", statusResp)
	}
	task := statusResp.Tasks[0]
	if task["id"] != createdTask.ID || task["status"] != model.TaskStatusSucceeded {
		t.Fatalf("unexpected status payload: %#v", task)
	}
	if _, ok := task["prompt"]; ok {
		t.Fatalf("status payload should not include static prompt field: %#v", task)
	}
	if task["result_thumb_path"] == "" || task["result_width"].(float64) != 1024 {
		t.Fatalf("status payload missing result fields: %#v", task)
	}
}

func TestTaskCreateIgnoresQualityField(t *testing.T) {
	t.Parallel()
	e, _, userBearer := setupAuthenticatedUser(t)

	taskResp := postMultipartTaskFields(t, e, userBearer, map[string]string{
		"prompt":  "一张赛博朋克城市海报",
		"size":    "2048x2048",
		"quality": "high",
	}, false)
	if taskResp.Code != http.StatusCreated {
		t.Fatalf("create task status = %d, body = %s", taskResp.Code, taskResp.Body.String())
	}
	var createdTask struct {
		Size    string `json:"size"`
		Quality string `json:"quality"`
	}
	decode(t, taskResp.Body.Bytes(), &createdTask)
	if createdTask.Size != "2048x2048" || createdTask.Quality != "auto" {
		t.Fatalf("unexpected task sizing response: %#v", createdTask)
	}
}

func TestTaskCreateValidatesRequestedSize(t *testing.T) {
	t.Parallel()
	e, _, userBearer := setupAuthenticatedUser(t)

	cases := map[string]string{
		"not-multiple":  "1025x1024",
		"too-long":      "3856x1024",
		"too-wide":      "3840x1024",
		"too-few-pixel": "256x256",
		"too-many":      "3840x3840",
	}
	for name, size := range cases {
		t.Run(name, func(t *testing.T) {
			taskResp := postMultipartTaskFields(t, e, userBearer, map[string]string{
				"prompt": "一张赛博朋克城市海报",
				"size":   size,
			}, false)
			if taskResp.Code != http.StatusBadRequest {
				t.Fatalf("create task with size %q status = %d, body = %s", size, taskResp.Code, taskResp.Body.String())
			}
		})
	}
}

func TestTaskCreateAllowsAutoSize(t *testing.T) {
	t.Parallel()
	e, _, userBearer := setupAuthenticatedUser(t)

	taskResp := postMultipartTaskFields(t, e, userBearer, map[string]string{
		"prompt": "一张赛博朋克城市海报",
		"size":   "auto",
	}, false)
	if taskResp.Code != http.StatusCreated {
		t.Fatalf("create task with auto size status = %d, body = %s", taskResp.Code, taskResp.Body.String())
	}
	var createdTask struct {
		Size string `json:"size"`
	}
	decode(t, taskResp.Body.Bytes(), &createdTask)
	if createdTask.Size != "auto" {
		t.Fatalf("created task size = %q, want auto", createdTask.Size)
	}
}

func TestFrontendFallbackMissingIndexReturns500(t *testing.T) {
	t.Parallel()
	e := echo.New()
	if err := server.RegisterFrontendFS(e, fstest.MapFS{
		"dist/app.js": &fstest.MapFile{Data: []byte("console.log('ok')")},
	}); err != nil {
		t.Fatal(err)
	}

	first := get(t, e, "/missing-route", "")
	if first.Code != http.StatusInternalServerError {
		t.Fatalf("fallback without index status = %d, body = %s", first.Code, first.Body.String())
	}
	second := get(t, e, "/another-missing-route", "")
	if second.Code != http.StatusInternalServerError {
		t.Fatalf("server should keep serving after missing index, status = %d, body = %s", second.Code, second.Body.String())
	}
}

type claimedTaskContext struct {
	Handler      http.Handler
	DB           *gorm.DB
	UserBearer   string
	RunnerBearer string
	RunnerID     string
	TaskID       string
}

func setupClaimedTask(t *testing.T) claimedTaskContext {
	t.Helper()
	e, db, userBearer := setupAuthenticatedUser(t)

	taskResp := postMultipartTask(t, e, userBearer)
	var createdTask struct {
		ID string `json:"id"`
	}
	decode(t, taskResp.Body.Bytes(), &createdTask)

	runner, err := service.CreateRunner(db, "runner-a")
	if err != nil {
		t.Fatal(err)
	}
	runnerBearer := "Bearer " + runner.Token
	register := postJSON(t, e, "/api/runner/runners/register", map[string]any{"name": "runner-a", "version": "0.1.0"}, runnerBearer)
	if register.Code != http.StatusOK {
		t.Fatalf("runner register status = %d, body = %s", register.Code, register.Body.String())
	}

	claim := postJSON(t, e, "/api/runner/runners/"+runner.RunnerID+"/tasks/claim", map[string]any{"limit": 1}, runnerBearer)
	if claim.Code != http.StatusOK {
		t.Fatalf("claim status = %d, body = %s", claim.Code, claim.Body.String())
	}

	return claimedTaskContext{
		Handler:      e,
		DB:           db,
		UserBearer:   userBearer,
		RunnerBearer: runnerBearer,
		RunnerID:     runner.RunnerID,
		TaskID:       createdTask.ID,
	}
}

func setupAuthenticatedUser(t *testing.T) (http.Handler, *gorm.DB, string) {
	t.Helper()
	dataDir := t.TempDir()
	db, err := database.Open(filepath.Join(dataDir, "imageforge.db"))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("test123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "admin", PasswordHash: string(hash)}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	e := server.New(config.Config{DataDir: dataDir, JWTSecret: "test-secret"}, db)

	login := postJSON(t, e, "/api/auth/login", map[string]any{"username": "admin", "password": "test123"}, "")
	var loginResp struct {
		Token string `json:"token"`
	}
	decode(t, login.Body.Bytes(), &loginResp)
	return e, db, "Bearer " + loginResp.Token
}

func postJSON(t *testing.T, e http.Handler, path string, body any, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	return postJSONFromIP(t, e, path, body, bearer, "")
}

func postJSONFromIP(t *testing.T, e http.Handler, path string, body any, bearer string, ip string) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if ip != "" {
		req.Header.Set(echo.HeaderXForwardedFor, ip)
	}
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func postMultipartTask(t *testing.T, e http.Handler, bearer string) *httptest.ResponseRecorder {
	return postMultipartTaskWithOptionalReference(t, e, bearer, false)
}

func postMultipartTaskWithReference(t *testing.T, e http.Handler, bearer string) *httptest.ResponseRecorder {
	return postMultipartTaskWithOptionalReference(t, e, bearer, true)
}

func postMultipartTaskWithReferenceImages(t *testing.T, e http.Handler, bearer string, count int) *httptest.ResponseRecorder {
	t.Helper()
	images := make([][]byte, 0, count)
	names := make([]string, 0, count)
	for i := 0; i < count; i++ {
		images = append(images, tinyPNGBytes(t))
		names = append(names, "reference.png")
	}
	return postMultipartTaskWithReferencePayloads(t, e, bearer, images, names)
}

func postMultipartTaskWithReferencePayloads(t *testing.T, e http.Handler, bearer string, images [][]byte, names []string) *httptest.ResponseRecorder {
	t.Helper()
	return postMultipartTaskFieldsAndReferences(t, e, bearer, map[string]string{
		"prompt":  "一张赛博朋克城市海报",
		"size":    "1024x1024",
		"quality": "auto",
	}, "reference_images", images, names)
}

func postMultipartTaskWithOptionalReference(t *testing.T, e http.Handler, bearer string, includeReference bool) *httptest.ResponseRecorder {
	t.Helper()
	return postMultipartTaskFields(t, e, bearer, map[string]string{
		"prompt":  "一张赛博朋克城市海报",
		"size":    "1024x1024",
		"quality": "auto",
	}, includeReference)
}

func postMultipartTaskFields(t *testing.T, e http.Handler, bearer string, fields map[string]string, includeReference bool) *httptest.ResponseRecorder {
	t.Helper()
	var images [][]byte
	var names []string
	if includeReference {
		images = [][]byte{tinyPNGBytes(t)}
		names = []string{"reference.png"}
	}
	return postMultipartTaskFieldsAndReferences(t, e, bearer, fields, "reference_image", images, names)
}

func postMultipartTaskFieldsAndReferences(t *testing.T, e http.Handler, bearer string, fields map[string]string, fieldName string, images [][]byte, names []string) *httptest.ResponseRecorder {
	t.Helper()
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	for i, image := range images {
		name := "reference.png"
		if i < len(names) && names[i] != "" {
			name = names[i]
		}
		part, err := writer.CreateFormFile(fieldName, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(image); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", bearer)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func postMultipartResult(t *testing.T, e http.Handler, path string, fields map[string]string, image []byte, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if image != nil {
		part, err := writer.CreateFormFile("image", "result.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(image); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", bearer)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func get(t *testing.T, e http.Handler, path string, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func waitForTaskStatus(t *testing.T, db *gorm.DB, taskID string, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var task model.Task
		if err := db.Where("id = ?", taskID).First(&task).Error; err != nil {
			t.Fatal(err)
		}
		if task.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	var task model.Task
	if err := db.Where("id = ?", taskID).First(&task).Error; err != nil {
		t.Fatal(err)
	}
	t.Fatalf("task status = %s, want %s", task.Status, want)
}

func decode(t *testing.T, data []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", string(data), err)
	}
}

func assertTaskImagePath(t *testing.T, got string, taskID string) {
	t.Helper()
	parts := strings.Split(got, "/")
	if len(parts) < 5 || parts[0] != "images" || parts[4] != taskID {
		t.Fatalf("image path = %q, want images/YYYY/MM/DD/%s/...", got, taskID)
	}
	if _, err := time.Parse("2006/01/02", strings.Join(parts[1:4], "/")); err != nil {
		t.Fatalf("image path = %q, date segment should be YYYY/MM/DD: %v", got, err)
	}
}

const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4z8DwHwAFAAH/iZk9HQAAAABJRU5ErkJggg=="

func tinyPNGBytes(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(tinyPNGBase64)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
