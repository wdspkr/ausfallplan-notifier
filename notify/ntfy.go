package notify

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultServer = "https://ntfy.sh"

// Ntfy posts notifications to a ntfy.sh-compatible server.
// Default server is https://ntfy.sh.
type Ntfy struct {
	Server string      // base URL, no trailing slash, e.g. "https://ntfy.sh"
	Topic  string      // unguessable topic the user has subscribed to on their phone
	Client *http.Client // optional override; nil → uses a 10s-timeout default
}

// NewNtfy creates a new Ntfy notifier. If server is empty, it defaults to
// "https://ntfy.sh".
func NewNtfy(server, topic string) *Ntfy {
	if server == "" {
		server = defaultServer
	}
	return &Ntfy{
		Server: server,
		Topic:  topic,
	}
}

// Send POSTs the notification to <Server>/<Topic>.
//
// Headers set:
//   - Content-Type: text/plain; charset=utf-8 (always)
//   - Title: n.Title (always set if non-empty)
//   - Tags: comma-joined n.Tags (only if len(Tags) > 0)
//
// Non-2xx responses return an error that includes the HTTP status code.
// Network errors and context cancellation are propagated as errors.
func (n *Ntfy) Send(ctx context.Context, msg Notification) error {
	client := n.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	url := n.Server + "/" + n.Topic

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(msg.Body))
	if err != nil {
		return fmt.Errorf("ntfy: create request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if msg.Title != "" {
		req.Header.Set("Title", msg.Title)
	}
	if len(msg.Tags) > 0 {
		req.Header.Set("Tags", strings.Join(msg.Tags, ","))
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ntfy: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy: unexpected status %d", resp.StatusCode)
	}

	return nil
}
