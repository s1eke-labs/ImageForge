package cli_test

import (
	"bytes"
	"encoding/json"
	"imageforge/backend/internal/cli"
	"imageforge/backend/internal/config"
	"imageforge/backend/internal/database"
	"imageforge/backend/internal/server"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestResetPasswordChangesHTTPLogin(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("IMAGEFORGE_DATA_DIR", dataDir)

	runCLI(t, "user", "create", "--username", "admin", "--password", "oldpass")

	db, err := database.Open(filepath.Join(dataDir, "imageforge.db"))
	if err != nil {
		t.Fatal(err)
	}
	e := server.New(config.Config{DataDir: dataDir, JWTSecret: "test-secret"}, db)

	initialLogin := login(t, e, "admin", "oldpass")
	if initialLogin.Code != http.StatusOK {
		t.Fatalf("initial login status = %d, body = %s", initialLogin.Code, initialLogin.Body.String())
	}
	var initialLoginResp struct {
		Token string `json:"token"`
	}
	decode(t, initialLogin.Body.Bytes(), &initialLoginResp)

	runCLI(t, "user", "reset-password", "--username", "admin", "--password", "newpass")

	oldLogin := login(t, e, "admin", "oldpass")
	if oldLogin.Code != http.StatusUnauthorized {
		t.Fatalf("old password status = %d, body = %s", oldLogin.Code, oldLogin.Body.String())
	}
	oldTokenTasks := get(t, e, "/api/tasks", "Bearer "+initialLoginResp.Token)
	if oldTokenTasks.Code != http.StatusUnauthorized {
		t.Fatalf("old token protected request status = %d, body = %s", oldTokenTasks.Code, oldTokenTasks.Body.String())
	}
	newLogin := login(t, e, "admin", "newpass")
	if newLogin.Code != http.StatusOK {
		t.Fatalf("new password status = %d, body = %s", newLogin.Code, newLogin.Body.String())
	}
	var newLoginResp struct {
		Token string `json:"token"`
	}
	decode(t, newLogin.Body.Bytes(), &newLoginResp)
	newTokenTasks := get(t, e, "/api/tasks", "Bearer "+newLoginResp.Token)
	if newTokenTasks.Code != http.StatusOK {
		t.Fatalf("new token protected request status = %d, body = %s", newTokenTasks.Code, newTokenTasks.Body.String())
	}
}

func TestRunnerCreateOutputsCredentialsForRunnerAPI(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("IMAGEFORGE_DATA_DIR", dataDir)

	output := runCLIOutput(t, "runner", "create", "--name", "runner-a", "--url", "http://example.test/api/runner")

	values := parseCLIKeyValues(output)
	if values["runner_id"] == "" || values["token"] == "" {
		t.Fatalf("runner credentials incomplete: %s", output)
	}
	if values["url"] != "http://example.test/api/runner" {
		t.Fatalf("runner url = %q", values["url"])
	}

	db, err := database.Open(filepath.Join(dataDir, "imageforge.db"))
	if err != nil {
		t.Fatal(err)
	}
	e := server.New(config.Config{DataDir: dataDir, JWTSecret: "test-secret"}, db)
	register := postJSON(t, e, "/api/runner/runners/register", map[string]any{"version": "0.1.0"}, "Bearer "+values["token"])
	if register.Code != http.StatusOK {
		t.Fatalf("runner register status = %d, body = %s", register.Code, register.Body.String())
	}
	list := runCLIOutput(t, "runner", "list")
	if !strings.Contains(list, values["runner_id"]) || !strings.Contains(list, "online") || !strings.Contains(list, "0.1.0") {
		t.Fatalf("runner list missing live runner data: %s", list)
	}
}

func TestRunnerListAndDelete(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("IMAGEFORGE_DATA_DIR", dataDir)

	output := runCLIOutput(t, "runner", "create", "--name", "runner-a")
	values := parseCLIKeyValues(output)
	runnerID := values["runner_id"]
	if runnerID == "" {
		t.Fatalf("runner_id missing: %s", output)
	}

	list := runCLIOutput(t, "runner", "list")
	if !strings.Contains(list, runnerID) || !strings.Contains(list, "runner-a") || !strings.Contains(list, "offline") {
		t.Fatalf("runner list missing runner data: %s", list)
	}

	deleteOutput := runCLIOutput(t, "runner", "delete", "--id", runnerID)
	if !strings.Contains(deleteOutput, "deleted runner "+runnerID) {
		t.Fatalf("delete output = %s", deleteOutput)
	}
	list = runCLIOutput(t, "runner", "list")
	if strings.Contains(list, runnerID) {
		t.Fatalf("deleted runner still listed: %s", list)
	}
}

func runCLI(t *testing.T, args ...string) {
	t.Helper()
	_ = runCLIOutput(t, args...)
}

func runCLIOutput(t *testing.T, args ...string) string {
	t.Helper()
	root := cli.NewRoot()
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	root.SetArgs(args)
	root.SetOut(out)
	root.SetErr(errOut)
	if err := root.Execute(); err != nil {
		t.Fatalf("%v\nstderr: %s", err, errOut.String())
	}
	return out.String()
}

func parseCLIKeyValues(output string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return values
}

func login(t *testing.T, h http.Handler, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func postJSON(t *testing.T, h http.Handler, path string, body any, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func get(t *testing.T, h http.Handler, path string, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, data []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", string(data), err)
	}
}
