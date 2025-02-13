package main

import (
	"fmt"
	"log/slog"
)

func infof(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...))
}

func warnf(format string, args ...any) {
	slog.Warn(fmt.Sprintf(format, args...))
}

func errorf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
}
