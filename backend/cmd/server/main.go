package main

import (
	"log"

	"imageforge/backend/internal/config"
	"imageforge/backend/internal/database"
	"imageforge/backend/internal/server"
)

func main() {
	cfg, err := config.Load(true)
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.Open(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}

	e := server.New(cfg, db)
	e.Logger.Fatal(e.Start(":" + cfg.Port))
}
