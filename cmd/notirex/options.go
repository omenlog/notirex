package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"time"
)

type options struct {
	kind     string
	message  string
	position string
	duration time.Duration
}

func parseOptions(args []string) (options, error) {
	flags := flag.NewFlagSet("notirex", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	var result options
	flags.StringVar(&result.kind, "kind", "ok", "notification kind: ok, warning, error, neutral, or info")
	flags.StringVar(&result.message, "message", "", "notification message")
	flags.StringVar(&result.position, "position", "bottom-right", "notification position: top-left, bottom-left, top-right, bottom-right")
	flags.DurationVar(&result.duration, "duration", defaultDuration, "how long to display the notification")

	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		return options{}, errors.New("unexpected positional arguments")
	}
	if result.duration <= 0 {
		return options{}, errors.New("duration must be greater than zero")
	}
	return result, nil
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "usage:")
	fmt.Fprintln(writer, "  notirex --message TEXT [--kind ok|warning|error|neutral|info] [--duration 1s] [--position top-left|top-right|bottom-right|bottom-left]")
}
