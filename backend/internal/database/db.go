package database

import (
	"os"
	"path/filepath"

	"imageforge/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

func Open(dbPath string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Dialector{
		DriverName: "sqlite",
		DSN:        dbPath,
	}, &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, err
	}
	if err := migrateProjectRunners(db); err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&model.User{}, &model.Runner{}, &model.Task{}); err != nil {
		return nil, err
	}
	return db, nil
}

func migrateProjectRunners(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Runner{}) {
		return nil
	}
	var columns []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info(runners)").Scan(&columns).Error; err != nil {
		return err
	}
	hasUserID := false
	for _, column := range columns {
		if column.Name == "user_id" {
			hasUserID = true
			break
		}
	}
	if !hasUserID {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			CREATE TABLE runners_new (
				id text PRIMARY KEY,
				name text NOT NULL,
				token_hash text NOT NULL,
				token_prefix text NOT NULL,
				status text NOT NULL DEFAULT "offline",
				version text,
				last_heartbeat_at datetime,
				created_at datetime
			)
		`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO runners_new (id, name, token_hash, token_prefix, status, version, last_heartbeat_at, created_at)
			SELECT id, name, token_hash, token_prefix, status, version, last_heartbeat_at, created_at FROM runners
		`).Error; err != nil {
			return err
		}
		if err := tx.Exec("DROP TABLE runners").Error; err != nil {
			return err
		}
		if err := tx.Exec("ALTER TABLE runners_new RENAME TO runners").Error; err != nil {
			return err
		}
		return nil
	})
}
