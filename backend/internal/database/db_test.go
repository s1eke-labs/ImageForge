package database_test

import (
	"path/filepath"
	"testing"

	"imageforge/backend/internal/database"
	"imageforge/backend/internal/model"
	"imageforge/backend/internal/service"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestOpenMigratesUserScopedRunners(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "imageforge.db")
	oldDB, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: dbPath}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := oldDB.Exec("CREATE TABLE users (id integer PRIMARY KEY AUTOINCREMENT, username text NOT NULL, password_hash text NOT NULL, created_at datetime)").Error; err != nil {
		t.Fatal(err)
	}
	if err := oldDB.Exec(`
		CREATE TABLE runners (
			id text PRIMARY KEY,
			user_id integer NOT NULL,
			name text NOT NULL,
			token_hash text NOT NULL,
			token_prefix text NOT NULL,
			status text NOT NULL DEFAULT "offline",
			version text,
			last_heartbeat_at datetime,
			created_at datetime
		)
	`).Error; err != nil {
		t.Fatal(err)
	}
	if err := oldDB.Exec("INSERT INTO users (id, username, password_hash, created_at) VALUES (1, 'admin', 'hash', datetime('now'))").Error; err != nil {
		t.Fatal(err)
	}
	if err := oldDB.Exec("INSERT INTO runners (id, user_id, name, token_hash, token_prefix, status, created_at) VALUES ('runner_old', 1, 'old', 'hash', 'rtkn_old', 'offline', datetime('now'))").Error; err != nil {
		t.Fatal(err)
	}

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	var columns []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info(runners)").Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}
	for _, column := range columns {
		if column.Name == "user_id" {
			t.Fatalf("user_id column was not removed: %#v", columns)
		}
	}

	var runner model.Runner
	if err := db.Where("id = ?", "runner_old").First(&runner).Error; err != nil {
		t.Fatal(err)
	}
	if runner.Name != "old" {
		t.Fatalf("preserved runner name = %q", runner.Name)
	}
	if _, err := service.CreateRunner(db, "new"); err != nil {
		t.Fatal(err)
	}
}
