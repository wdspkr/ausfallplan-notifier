package notify

import (
	"context"
	"fmt"
	"io"
)

// Notification is the payload sent to a Notifier.
type Notification struct {
	Title string
	Body  string
	Tags  []string // sent as comma-separated "Tags:" header; empty → omitted
}

// Notifier is implemented by anything that can deliver a Notification.
type Notifier interface {
	Send(ctx context.Context, n Notification) error
}

// Logger writes notifications to W as plain text — used as a dry-run fallback
// when NTFY_TOPIC is unset, so users can run `check` without configuring ntfy.
type Logger struct {
	W io.Writer
}

func (l *Logger) Send(ctx context.Context, n Notification) error {
	_, err := fmt.Fprintf(l.W, "[notify dry-run] %s | %s\n", n.Title, n.Body)
	return err
}

// Stub records notifications in memory; for tests only.
type Stub struct {
	Sent []Notification
	Err  error // if non-nil, Send returns it without recording
}

func (s *Stub) Send(ctx context.Context, n Notification) error {
	if s.Err != nil {
		return s.Err
	}
	s.Sent = append(s.Sent, n)
	return nil
}
