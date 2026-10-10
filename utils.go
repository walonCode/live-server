package main

import (
	"fmt"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	gray    = "\033[90m"
)

const reloadScript = `<script> new EventSource("/events").onmessage=function(){ location.reload() } </script>`

const Delay = 80 * time.Millisecond

var useColor = detectColor()

// Color only when stderr is a terminal and NO_COLOR is not set.
func detectColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func paint(color, s string) string {
	if !useColor {
		return s
	}
	return color + s + reset
}

func logf(color, tag, format string, args ...any) {
	ts := time.Now().Format("15:04:05")
	fmt.Fprintf(os.Stderr, "%s %s %s\n",
		paint(gray, ts),
		paint(bold+color, fmt.Sprintf("%-8s", tag)),
		fmt.Sprintf(format, args...),
	)
}

func fatalf(format string, args ...any) {
	logf(red, "FATAL", format, args...)
	os.Exit(1)
}

func opColor(e fsnotify.Event) string {
	switch {
	case e.Has(fsnotify.Create):
		return green
	case e.Has(fsnotify.Write):
		return yellow
	case e.Has(fsnotify.Remove):
		return red
	case e.Has(fsnotify.Rename):
		return magenta
	default:
		return gray
	}
}