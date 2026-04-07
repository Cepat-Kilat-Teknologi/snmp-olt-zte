package main

import (
	"context"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/app"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load .env file if present (ignored in production when file doesn't exist)
	if err := godotenv.Load(); err != nil {
		log.Debug().Msg("No .env file found, using environment variables")
	}

	server := app.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
