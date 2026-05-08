package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
	"github.com/wdspkr/ausfallplan-notifier/diff"
	"github.com/wdspkr/ausfallplan-notifier/fetch"
	"github.com/wdspkr/ausfallplan-notifier/store"
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

	stateFile := os.Getenv("STATE_FILE")
	if stateFile == "" {
		stateFile = "state.json"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	body, err := fetch.Fetch(ctx, url)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	next, err := ausfallplan.Parse(body)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	fileStore := store.NewFileStore(stateFile)

	prev, err := fileStore.Load(ctx)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	added := diff.Compute(prev, next)

	if len(added.Entries) == 0 && len(added.Infos) == 0 {
		fmt.Println("Keine neuen Einträge.")
	} else {
		fmt.Printf("Neue Einträge (%d):\n", len(added.Entries))
		for _, e := range added.Entries {
			fmt.Printf("  %s\n", formatEntry(e))
		}
		fmt.Printf("Neue Informationen (%d):\n", len(added.Infos))
		for _, inf := range added.Infos {
			fmt.Printf("  %s\n", inf.Text)
		}
	}

	if err := fileStore.Save(ctx, next); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	return nil
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
