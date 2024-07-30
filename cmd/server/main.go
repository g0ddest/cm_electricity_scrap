package main

import (
	"cm_electricity_scrap/internal/config"
	"cm_electricity_scrap/internal/handlers"
	"log"
	"time"
)

func main() {

	cfg := config.LoadConfig()

	for {
		err := handlers.ScrapAndProcess(cfg)
		if err != nil {
			log.Printf("Error during scrap and process: %v", err)
		}
		time.Sleep(2 * time.Hour)
	}
}
