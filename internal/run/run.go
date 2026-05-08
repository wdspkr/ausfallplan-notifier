package run

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
	"github.com/wdspkr/ausfallplan-notifier/diff"
	"github.com/wdspkr/ausfallplan-notifier/fetch"
	"github.com/wdspkr/ausfallplan-notifier/notify"
	"github.com/wdspkr/ausfallplan-notifier/store"
)

// Options holds everything Check needs. Both cmd/local and cmd/lambda
// construct this from env vars and pass it in.
type Options struct {
	URL string // AUSFALL_URL

	StoreBackend string // "file" (default) or "dynamo"
	StateFile    string // FileStore path (default "state.json")
	DDBTable     string // DynamoDB table name (default "ausfallplan-state")
	DDBEndpoint  string // optional, for DynamoDB Local

	NtfyTopic  string // empty → dry-run (Logger writes to stderr)
	NtfyServer string // empty → "https://ntfy.sh"

	Blacklist []string

	LogWriter io.Writer // human-readable output destination (stdout in CLI)

	SelfNotifyOnError bool           // when true, structural errors trigger an urgent ntfy
	Notifier          notify.Notifier // test seam — when non-nil, used instead of constructing one
}

// Check runs one fetch → parse → diff → filter → notify → save pass.
// On any error past the fetch, returns the error WITHOUT updating state
// (so the next run retries).
//
// Structural errors (parse, store-load, store-save) trigger a self-notification
// when opts.SelfNotifyOnError is true. Transient errors (fetch, notify send)
// are returned silently — the next scheduled run will retry.
func Check(ctx context.Context, opts Options) error {
	if opts.URL == "" {
		return fmt.Errorf("run: URL is empty")
	}

	notifier := opts.Notifier
	if notifier == nil {
		notifier = makeNotifier(opts)
	}

	st, err := makeStore(ctx, opts)
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}

	// --- fetch (transient — no self-notify on failure) ---
	body, err := fetch.Fetch(ctx, opts.URL)
	if err != nil {
		return fmt.Errorf("ausfallplan-notifier: fetch: %w", err)
	}

	// --- parse (structural — self-notify on failure) ---
	next, err := ausfallplan.Parse(body)
	if err != nil {
		return structuralError(ctx, opts, notifier, "parse", err)
	}

	// --- store-load (structural — self-notify on failure) ---
	prev, err := st.Load(ctx)
	if err != nil {
		return structuralError(ctx, opts, notifier, "store-load", err)
	}

	added := diff.Compute(prev, next)

	filteredEntries := ausfallplan.Filter(added.Entries, opts.Blacklist)

	if len(filteredEntries) == 0 && len(added.Infos) == 0 {
		fmt.Fprintln(opts.LogWriter, "Keine neuen Einträge.")
		// Still update state on no-op runs so removals are tracked.
		if err := st.Save(ctx, next); err != nil {
			return structuralError(ctx, opts, notifier, "store-save", err)
		}
		return nil
	}

	// Print and notify for each filtered entry.
	fmt.Fprintf(opts.LogWriter, "Neue Einträge (%d):\n", len(filteredEntries))
	for _, e := range filteredEntries {
		fmt.Fprintf(opts.LogWriter, "  %s\n", formatEntry(e))
		if err := notifier.Send(ctx, entryNotification(e)); err != nil {
			// transient — same channel just failed; log and return, no self-notify
			return fmt.Errorf("ausfallplan-notifier: notify: %w", err)
		}
	}

	// Print and notify for each info.
	fmt.Fprintf(opts.LogWriter, "Neue Informationen (%d):\n", len(added.Infos))
	for _, inf := range added.Infos {
		fmt.Fprintf(opts.LogWriter, "  %s\n", inf.Text)
		if err := notifier.Send(ctx, infoNotification(inf)); err != nil {
			// transient — same channel just failed; log and return, no self-notify
			return fmt.Errorf("ausfallplan-notifier: notify: %w", err)
		}
	}

	// Only save state after all notifications succeeded.
	// Save the raw, unfiltered snapshot — state represents the page, not our
	// notification view. This means a future blacklist change won't retroactively
	// re-notify on entries that were already seen on the page.
	if err := st.Save(ctx, next); err != nil {
		return structuralError(ctx, opts, notifier, "store-save", err)
	}

	return nil
}

// structuralError wraps err with stage context, optionally fires a self-notify,
// and always returns the wrapped error to the caller.
func structuralError(ctx context.Context, opts Options, notifier notify.Notifier, stage string, err error) error {
	wrapped := fmt.Errorf("ausfallplan-notifier: %s: %w", stage, err)
	if opts.SelfNotifyOnError {
		selfNotif := notify.Notification{
			Title:    fmt.Sprintf("Ausfallplan-Notifier: Fehler in %s", stage),
			Body:     err.Error(),
			Tags:     []string{"warning"},
			Priority: 5,
		}
		if sendErr := notifier.Send(ctx, selfNotif); sendErr != nil {
			fmt.Fprintf(opts.LogWriter, "self-notify failed: %v\n", sendErr)
		}
	}
	return wrapped
}

// makeStore selects and initialises a Store based on opts.StoreBackend.
//
//   - "" or "file" → FileStore backed by opts.StateFile (default "state.json").
//   - "dynamo"     → DynamoStore backed by opts.DDBTable (default "ausfallplan-state"),
//     optionally pointed at opts.DDBEndpoint for DynamoDB Local.
//   - anything else → error.
func makeStore(ctx context.Context, opts Options) (store.Store, error) {
	switch opts.StoreBackend {
	case "", "file":
		stateFile := opts.StateFile
		if stateFile == "" {
			stateFile = "state.json"
		}
		return store.NewFileStore(stateFile), nil

	case "dynamo":
		table := opts.DDBTable
		if table == "" {
			table = "ausfallplan-state"
		}

		cfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("load AWS config: %w", err)
		}

		client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			if opts.DDBEndpoint != "" {
				o.BaseEndpoint = aws.String(opts.DDBEndpoint)
			}
		})

		return store.NewDynamoStore(client, table), nil

	default:
		return nil, fmt.Errorf("unknown STATE_BACKEND: %q", opts.StoreBackend)
	}
}

// makeNotifier selects a Notifier based on opts.
// makeNotifier returns a real ntfy notifier when NtfyTopic is set, otherwise a
// dry-run Logger writing to stderr (with a one-line stderr warning). LogWriter
// is reserved for the pipeline's pretty output so callers piping stdout aren't
// polluted with dry-run noise. Lambda routes both std streams to CloudWatch
// so this has no effect on cloud behavior.
func makeNotifier(opts Options) notify.Notifier {
	if opts.NtfyTopic == "" {
		fmt.Fprintln(os.Stderr, "warning: NTFY_TOPIC is not set — notifications are in dry-run mode (printed to stderr only)")
		return &notify.Logger{W: os.Stderr}
	}
	return notify.NewNtfy(opts.NtfyServer, opts.NtfyTopic)
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

