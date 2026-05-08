package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
	"github.com/wdspkr/ausfallplan-notifier/config"
	"github.com/wdspkr/ausfallplan-notifier/diff"
	"github.com/wdspkr/ausfallplan-notifier/fetch"
	"github.com/wdspkr/ausfallplan-notifier/notify"
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

	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.json"
	}

	notifier := makeNotifier()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	st, err := makeStore(ctx)
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}

	body, err := fetch.Fetch(ctx, url)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	next, err := ausfallplan.Parse(body)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	prev, err := st.Load(ctx)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	added := diff.Compute(prev, next)

	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	filteredEntries := ausfallplan.Filter(added.Entries, cfg.Blacklist)

	if len(filteredEntries) == 0 && len(added.Infos) == 0 {
		fmt.Println("Keine neuen Einträge.")
		// Still update state on no-op runs so removals are tracked.
		if err := st.Save(ctx, next); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
		return nil
	}

	// Print and notify for each filtered entry.
	fmt.Printf("Neue Einträge (%d):\n", len(filteredEntries))
	for _, e := range filteredEntries {
		fmt.Printf("  %s\n", formatEntry(e))
		if err := notifier.Send(ctx, entryNotification(e)); err != nil {
			return fmt.Errorf("notify entry: %w", err)
		}
	}

	// Print and notify for each info.
	fmt.Printf("Neue Informationen (%d):\n", len(added.Infos))
	for _, inf := range added.Infos {
		fmt.Printf("  %s\n", inf.Text)
		if err := notifier.Send(ctx, infoNotification(inf)); err != nil {
			return fmt.Errorf("notify info: %w", err)
		}
	}

	// Only save state after all notifications succeeded.
	// Save the raw, unfiltered snapshot — state represents the page, not our
	// notification view. This means a future blacklist change won't retroactively
	// re-notify on entries that were already seen on the page.
	if err := st.Save(ctx, next); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	return nil
}

// makeStore selects and initialises a Store based on the STATE_BACKEND env var.
//
//   - "" or "file" → FileStore backed by STATE_FILE (default "state.json").
//   - "dynamo"     → DynamoStore backed by DDB_TABLE (default "ausfallplan-state"),
//     optionally pointed at DDB_ENDPOINT for DynamoDB Local.
//   - anything else → error.
func makeStore(ctx context.Context) (store.Store, error) {
	backend := os.Getenv("STATE_BACKEND")
	switch backend {
	case "", "file":
		stateFile := os.Getenv("STATE_FILE")
		if stateFile == "" {
			stateFile = "state.json"
		}
		return store.NewFileStore(stateFile), nil

	case "dynamo":
		table := os.Getenv("DDB_TABLE")
		if table == "" {
			table = "ausfallplan-state"
		}
		endpoint := os.Getenv("DDB_ENDPOINT")

		cfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("load AWS config: %w", err)
		}

		client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			if endpoint != "" {
				o.BaseEndpoint = aws.String(endpoint)
			}
		})

		return store.NewDynamoStore(client, table), nil

	default:
		return nil, fmt.Errorf("unknown STATE_BACKEND: %q", backend)
	}
}

// makeNotifier selects a Notifier based on environment variables.
// If NTFY_TOPIC is set, it returns a real ntfy.sh notifier.
// Otherwise it returns a Logger (dry-run) and prints a warning to stderr.
func makeNotifier() notify.Notifier {
	topic := os.Getenv("NTFY_TOPIC")
	if topic == "" {
		fmt.Fprintln(os.Stderr, "warning: NTFY_TOPIC is not set — notifications are in dry-run mode (printed to stderr only)")
		return &notify.Logger{W: os.Stderr}
	}

	server := os.Getenv("NTFY_SERVER")
	return notify.NewNtfy(server, topic)
}

// entryNotification builds a Notification for a single Ausfallplan Entry.
func entryNotification(e ausfallplan.Entry) notify.Notification {
	body := fmt.Sprintf("%s · %s · %s",
		e.Day.Format("Mon, 02.01.2006"),
		e.Hour,
		e.Information,
	)
	// Trim trailing separator when Information is empty.
	body = strings.TrimRight(body, " ·")
	body = strings.TrimRight(body, " ")
	return notify.Notification{
		Title: e.Class,
		Body:  body,
		Tags:  []string{"school"},
	}
}

// infoNotification builds a Notification for a single Aktuelle-Informationen Info.
func infoNotification(i ausfallplan.Info) notify.Notification {
	return notify.Notification{
		Title: "Aktuelle Information",
		Body:  i.Text,
		Tags:  []string{"school"},
	}
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
