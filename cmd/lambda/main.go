package main

import (
	"context"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/wdspkr/ausfallplan-notifier/internal/run"
)

func handler(ctx context.Context) error {
	opts := run.Options{
		URL:               os.Getenv("AUSFALL_URL"),
		StoreBackend:      os.Getenv("STATE_BACKEND"),
		StateFile:         os.Getenv("STATE_FILE"),
		DDBTable:          os.Getenv("DDB_TABLE"),
		DDBEndpoint:       os.Getenv("DDB_ENDPOINT"),
		NtfyTopic:         os.Getenv("NTFY_TOPIC"),
		NtfyServer:        os.Getenv("NTFY_SERVER"),
		Blacklist:         parseBlacklist(os.Getenv("BLACKLIST")),
		LogWriter:         os.Stderr, // CloudWatch captures both std streams; stderr is the convention for handler logs
		SelfNotifyOnError: os.Getenv("SELF_NOTIFY") == "true",
	}
	return run.Check(ctx, opts)
}

func parseBlacklist(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func main() {
	lambda.Start(handler)
}
