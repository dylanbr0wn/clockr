package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// rotateMaxSizeMB is mid-range of the 5–10MB size-based rotation target.
	rotateMaxSizeMB = 8
	// rotateMaxBackups keeps 1–2 rotated files alongside the active log.
	rotateMaxBackups = 2
)

// Open builds a redacting zerolog.Logger that always writes JSON to a
// size-rotating file at path. When console is true (wails dev), it also mirrors
// human-readable lines to stderr via MultiLevelWriter.
//
// The returned Closer flushes/closes the rotating file writer; call it on
// shutdown. Level defaults to info when empty.
func Open(path, level string, console bool) (zerolog.Logger, io.Closer, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return zerolog.Logger{}, nil, fmt.Errorf("log path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return zerolog.Logger{}, nil, fmt.Errorf("create log directory: %w", err)
	}

	file := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    rotateMaxSizeMB,
		MaxBackups: rotateMaxBackups,
		LocalTime:  true,
	}

	var w io.Writer = file
	if console {
		w = zerolog.MultiLevelWriter(
			file,
			zerolog.ConsoleWriter{Out: os.Stderr},
		)
	}

	lvl, err := parseLevel(level)
	if err != nil {
		_ = file.Close()
		return zerolog.Logger{}, nil, err
	}

	return New(w).Level(lvl), file, nil
}

func parseLevel(level string) (zerolog.Level, error) {
	level = strings.ToLower(strings.TrimSpace(level))
	if level == "" {
		return zerolog.InfoLevel, nil
	}
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		return zerolog.Disabled, fmt.Errorf("log level %q: %w", level, err)
	}
	return lvl, nil
}
