package run_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wdspkr/ausfallplan-notifier/internal/run"
)

func TestCheck_HappyPath(t *testing.T) {
	// Load the fixture HTML that has two entries: 4c and 3b · Ukulele.
	fixture, err := os.ReadFile("../../ausfallplan/testdata/header_in_tbody.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	// Spin up a test HTTP server returning the fixture HTML.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(fixture)
	}))
	defer srv.Close()

	stateDir := t.TempDir()
	stateFile := filepath.Join(stateDir, "state.json")

	var buf bytes.Buffer
	opts := run.Options{
		URL:       srv.URL,
		StateFile: stateFile,
		// NtfyTopic empty → Logger dry-run
		LogWriter: &buf,
		Blacklist: nil, // no filtering
	}

	// --- First run: should see both entries. ---
	if err := run.Check(context.Background(), opts); err != nil {
		t.Fatalf("first Check: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "3b") || !strings.Contains(out, "Ukulele") {
		t.Errorf("expected '3b · Ukulele' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "4c") {
		t.Errorf("expected '4c' in output, got:\n%s", out)
	}

	// State file must exist and be non-empty.
	info, err := os.Stat(stateFile)
	if err != nil {
		t.Fatalf("state file missing after first run: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("state file is empty after first run")
	}

	// --- Second run: idempotent — no new entries. ---
	var buf2 bytes.Buffer
	opts.LogWriter = &buf2
	if err := run.Check(context.Background(), opts); err != nil {
		t.Fatalf("second Check: %v", err)
	}

	out2 := buf2.String()
	if !strings.Contains(out2, "Keine neuen Einträge.") {
		t.Errorf("expected 'Keine neuen Einträge.' on second run, got:\n%s", out2)
	}
}

func TestCheck_EmptyURL(t *testing.T) {
	var buf bytes.Buffer
	opts := run.Options{
		URL:       "",
		LogWriter: &buf,
	}
	err := run.Check(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
	if !strings.Contains(err.Error(), "URL is empty") {
		t.Errorf("expected 'URL is empty' in error, got: %v", err)
	}
}
