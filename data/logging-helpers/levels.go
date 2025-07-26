package logginghelpers

import "log/slog"

const (
	// Level Debug -4
	LevelReportIO slog.Level = -2
	// Level Info 0
	// Level Warn 4
	// Level Error 8
	LevelBrokenProcess slog.Level = 12
)
