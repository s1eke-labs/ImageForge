package config

import (
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	Port      string
	DataDir   string
	DBPath    string
	JWTSecret string
}

func Load(requireJWT bool) (Config, error) {
	dataDir := getenv("IMAGEFORGE_DATA_DIR", "./data")
	cfg := Config{
		Port:      getenv("IMAGEFORGE_PORT", "8020"),
		DataDir:   dataDir,
		DBPath:    filepath.Join(dataDir, "imageforge.db"),
		JWTSecret: os.Getenv("IMAGEFORGE_JWT_SECRET"),
	}
	if requireJWT && cfg.JWTSecret == "" {
		return cfg, errors.New("IMAGEFORGE_JWT_SECRET is required for JWT signing")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
