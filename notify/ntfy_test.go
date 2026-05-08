package notify_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wdspkr/ausfallplan-notifier/notify"
)

// capture is a helper that records the last received request.
type capture struct {
	method      string
	path        string
	body        string
	contentType string
	title       string
	tags        []string
}

func newCaptureServer(t *testing.T, statusCode int) (*httptest.Server, *capture) {
	t.Helper()
	cap := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.method = r.Method
		cap.path = r.URL.Path
		cap.contentType = r.Header.Get("Content-Type")
		cap.title = r.Header.Get("Title")
		cap.tags = r.Header.Values("Tags")
		b, _ := io.ReadAll(r.Body)
		cap.body = string(b)
		w.WriteHeader(statusCode)
	}))
	return srv, cap
}

func TestNtfy_Send_HappyPath(t *testing.T) {
	srv, cap := newCaptureServer(t, http.StatusOK)
	defer srv.Close()

	n := notify.NewNtfy(srv.URL, "mytopic")
	msg := notify.Notification{
		Title: "3d",
		Body:  "Mo, 11.05.2026 · 6. Stunde · Deutsch",
		Tags:  []string{"school", "warning"},
	}

	if err := n.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if cap.method != http.MethodPost {
		t.Errorf("method = %q; want POST", cap.method)
	}
	if cap.path != "/mytopic" {
		t.Errorf("path = %q; want /mytopic", cap.path)
	}
	if !strings.HasPrefix(cap.contentType, "text/plain") {
		t.Errorf("Content-Type = %q; want prefix text/plain", cap.contentType)
	}
	if cap.title != msg.Title {
		t.Errorf("Title header = %q; want %q", cap.title, msg.Title)
	}
	wantTags := "school,warning"
	if len(cap.tags) == 0 || cap.tags[0] != wantTags {
		t.Errorf("Tags header = %v; want [%q]", cap.tags, wantTags)
	}
	if cap.body != msg.Body {
		t.Errorf("body = %q; want %q", cap.body, msg.Body)
	}
}

func TestNtfy_Send_OmitsTagsHeaderWhenEmpty(t *testing.T) {
	srv, cap := newCaptureServer(t, http.StatusOK)
	defer srv.Close()

	n := notify.NewNtfy(srv.URL, "mytopic")
	msg := notify.Notification{
		Title: "Test",
		Body:  "no tags here",
		Tags:  []string{},
	}

	if err := n.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if vals := cap.tags; len(vals) != 0 {
		t.Errorf("Tags header should be absent; got %v", vals)
	}
}

func TestNtfy_Send_5xxReturnsError(t *testing.T) {
	srv, _ := newCaptureServer(t, http.StatusInternalServerError)
	defer srv.Close()

	n := notify.NewNtfy(srv.URL, "mytopic")
	err := n.Send(context.Background(), notify.Notification{Title: "t", Body: "b"})
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") && !strings.Contains(err.Error(), "status") {
		t.Errorf("error %q does not mention 500 or status", err.Error())
	}
}

func TestNtfy_Send_4xxReturnsError(t *testing.T) {
	srv, _ := newCaptureServer(t, http.StatusNotFound)
	defer srv.Close()

	n := notify.NewNtfy(srv.URL, "mytopic")
	err := n.Send(context.Background(), notify.Notification{Title: "t", Body: "b"})
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}

func TestNtfy_Send_ContextCancelled(t *testing.T) {
	srv, _ := newCaptureServer(t, http.StatusOK)
	defer srv.Close()

	n := notify.NewNtfy(srv.URL, "mytopic")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	err := n.Send(ctx, notify.Notification{Title: "t", Body: "b"})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestNtfy_Send_DefaultServer(t *testing.T) {
	n := notify.NewNtfy("", "topic")
	if n.Server != "https://ntfy.sh" {
		t.Errorf("Server = %q; want %q", n.Server, "https://ntfy.sh")
	}
}
