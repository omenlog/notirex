package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"notirex/internal/notification"
)

type showNotification func(notification.Notification, time.Duration) error

func main() {
	os.Exit(app(os.Args[1:], os.Stdout, os.Stderr, runNativeNotification))
}

func app(args []string, stdout, stderr io.Writer, show showNotification) int {
	// invalid arguments
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	// parsing help argument
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		printUsage(stdout)
		return 0
	}

	// parsing command options
	options, err := parseOptions(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	// building notification object
	notice, err := notification.New(options.kind, options.message, options.position)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	if err := show(notice, options.duration); err != nil {
		fmt.Fprintf(stderr, "show notification: %v\n", err)
		return 1
	}

	return 0
}
