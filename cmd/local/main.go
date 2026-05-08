package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
	"github.com/wdspkr/ausfallplan-notifier/config"
	"github.com/wdspkr/ausfallplan-notifier/fetch"
	"github.com/wdspkr/ausfallplan-notifier/internal/run"
)

func main() {
	// Load .env best-effort; ignore error (not present in CI/Lambda).
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "fetch":
		if err := runFetch(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "check":
		if err := runCheck(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(2)
	}
}

func runFetch() error {
	url := os.Getenv("AUSFALL_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "error: AUSFALL_URL environment variable is not set")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	body, err := fetch.Fetch(ctx, url)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	snap, err := ausfallplan.Parse(body)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	fmt.Printf("Ausfall (%d entries):\n", len(snap.Entries))
	for _, e := range snap.Entries {
		fmt.Printf("  %s\n", formatEntry(e))
	}

	fmt.Printf("Aktuelle Informationen (%d entries):\n", len(snap.Infos))
	for _, inf := range snap.Infos {
		fmt.Printf("  %s\n", inf.Text)
	}

	return nil
}

func runCheck() error {
	url := os.Getenv("AUSFALL_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "error: AUSFALL_URL environment variable is not set")
		os.Exit(2)
	}

	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.json"
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	opts := run.Options{
		URL:          url,
		StoreBackend: os.Getenv("STATE_BACKEND"),
		StateFile:    os.Getenv("STATE_FILE"),
		DDBTable:     os.Getenv("DDB_TABLE"),
		DDBEndpoint:  os.Getenv("DDB_ENDPOINT"),
		NtfyTopic:    os.Getenv("NTFY_TOPIC"),
		NtfyServer:   os.Getenv("NTFY_SERVER"),
		Blacklist:    cfg.Blacklist,
		LogWriter:    os.Stdout,
	}

	return run.Check(ctx, opts)
}

// formatEntry formats an Entry consistently across subcommands.
func formatEntry(e ausfallplan.Entry) string {
	return fmt.Sprintf("%s · %s · %s · %s",
		e.Day.Format("Mon, 02.01.2006"),
		e.Hour,
		e.Class,
		e.Information,
	)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: local <subcommand>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Subcommands:")
	fmt.Fprintln(os.Stderr, "  fetch   Fetch and parse the Ausfall page, print a summary")
	fmt.Fprintln(os.Stderr, "  check   Fetch the Ausfall page, diff against last saved state, print additions")
}
