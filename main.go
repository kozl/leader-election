package main

import (
	"log/slog"
	"os"

	"github.com/kozl/leader-election/internal"
)

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

func main() {
	app, err := internal.NewApp(logger)
	if err != nil {
		fatal(err)
	}
	if err := app.Run(); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	logger.Error("Fatal error, exiting", "error", err)
	os.Exit(1)
}
