package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetch200OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))
	defer srv.Close()

	body, err := Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if string(body) != "hello world" {
		t.Errorf("expected body %q, got %q", "hello world", string(body))
	}
}

func TestFetch404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected an error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "status") {
		t.Errorf("error should mention 404 or status, got: %v", err)
	}
}

func TestFetch500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected an error for 500, got nil")
	}
}

func TestFetchCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until the request context is done.
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := Fetch(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected an error for cancelled context, got nil")
	}
}
