package logger

import (
	"io"
	"log/slog"
	"os"
	"rulemanager/config"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Setup initializes the global logger based on the configuration.
func Setup(cfg config.LoggingConfig) {
	var w io.Writer = os.Stdout

	// Configure output
	if strings.ToLower(cfg.Output) == "file" && cfg.FilePath != "" {
		w = &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSize, // megabytes
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge, // days
			Compress:   cfg.Compress,
		}
	}

	// Configure level
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	// Configure handler (JSON or Text)
	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	// Set global logger
	logger := slog.New(handler)
	slog.SetDefault(logger)
}
