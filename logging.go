package main

import (
	"log/slog"
	"os"
)

// logLevel starts at Info and drops to Debug once the config enables Debug.
var logLevel = new(slog.LevelVar)

// InitializeLogging makes slog (and the standard log package, which slog
// redirects) write key=value lines to stderr.
func InitializeLogging() {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))
}

func SetDebugLogging(on bool) {
	if on {
		logLevel.Set(slog.LevelDebug)
	} else {
		logLevel.Set(slog.LevelInfo)
	}
}
