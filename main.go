package main

import (
	"context"
	"fmt"
	"keyframe/internal"
	"log/slog"
	"os"
	"time"

	"github.com/urfave/cli/v3"

	"keyframe/cmd"
)

func main() {
	if err := setupLogger(); err != nil {
		_, err := fmt.Fprintln(os.Stderr, err)
		if err != nil {
			return
		}
		os.Exit(1)
	}

	if err := internal.EnableDPIAwareness(); err != nil {
		slog.Warn("enable DPI awareness failed", "error", err)
	}

	app := &cli.Command{
		Name:  "keyframe",
		Usage: "A lightweight screen sharing tool",
		Commands: []*cli.Command{
			cmd.LocalCommand,
			cmd.RemoteCommand,
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func setupLogger() error {
	filename := time.Now().Format("2006-01-02_15-04-05.000") + ".log"

	file, err := os.OpenFile(
		filename,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	slog.SetDefault(logger)
	return nil
}
