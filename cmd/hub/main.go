package main

import (
	"log/slog"
	"os"

	"github.com/AmooVPM/hub/internal/app"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	runner, err := app.New(logger)
	if err != nil {
		logger.Error("startup failed", "error", err)
		os.Exit(1)
	}

	if err := runner.Run(); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
