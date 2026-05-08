package store_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
	"github.com/wdspkr/ausfallplan-notifier/store"
)

var (
	day1 = time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	day2 = time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)

	testSnap = ausfallplan.Snapshot{
		Entries: []ausfallplan.Entry{
			{Day: day2, Hour: "1. Stunde", Class: "6b", Information: "Vertretung"},
			{Day: day1, Hour: "3. Stunde", Class: "3d", Information: "Ausfall"},
		},
		Infos: []ausfallplan.Info{
			{Text: "Schulausflug 4c"},
			{Text: "Morgen kein Sport"},
		},
	}
)

func TestFileStore_LoadMissingFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	fs := store.NewFileStore(filepath.Join(dir, "state.json"))

	snap, err := fs.Load(context.Background())
	if err != nil {
		t.Fatalf("expected nil error on missing file, got %v", err)
	}
	if len(snap.Entries) != 0 {
		t.Errorf("expected empty Entries, got %d", len(snap.Entries))
	}
	if len(snap.Infos) != 0 {
		t.Errorf("expected empty Infos, got %d", len(snap.Infos))
	}
}

func TestFileStore_SaveAndLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	fs := store.NewFileStore(filepath.Join(dir, "state.json"))
	ctx := context.Background()

	if err := fs.Save(ctx, testSnap); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := fs.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(got.Entries) != len(testSnap.Entries) {
		t.Fatalf("expected %d entries, got %d", len(testSnap.Entries), len(got.Entries))
	}
	if len(got.Infos) != len(testSnap.Infos) {
		t.Fatalf("expected %d infos, got %d", len(testSnap.Infos), len(got.Infos))
	}

	// Content should match — but because Save sorts, the order will differ from
	// testSnap's insertion order. Check by Class and Text fields rather than index.
	classSet := make(map[string]bool)
	for _, e := range got.Entries {
		classSet[e.Class] = true
	}
	for _, e := range testSnap.Entries {
		if !classSet[e.Class] {
			t.Errorf("entry with class %q not found after roundtrip", e.Class)
		}
	}

	textSet := make(map[string]bool)
	for _, i := range got.Infos {
		textSet[i.Text] = true
	}
	for _, i := range testSnap.Infos {
		if !textSet[i.Text] {
			t.Errorf("info with text %q not found after roundtrip", i.Text)
		}
	}
}

func TestFileStore_Save_SortsContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	fs := store.NewFileStore(path)
	ctx := context.Background()

	if err := fs.Save(ctx, testSnap); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := fs.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Entries should be sorted by Day first (day1 before day2).
	if len(got.Entries) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(got.Entries))
	}
	if !got.Entries[0].Day.Before(got.Entries[1].Day) && got.Entries[0].Day != got.Entries[1].Day {
		t.Errorf("entries not sorted by Day: first=%v, second=%v",
			got.Entries[0].Day, got.Entries[1].Day)
	}

	// Infos should be sorted by Text ("Morgen kein Sport" < "Schulausflug 4c").
	if len(got.Infos) < 2 {
		t.Fatalf("expected at least 2 infos, got %d", len(got.Infos))
	}
	if got.Infos[0].Text > got.Infos[1].Text {
		t.Errorf("infos not sorted by Text: first=%q, second=%q", got.Infos[0].Text, got.Infos[1].Text)
	}
}

func TestFileStore_SaveTwice_ByteIdentical(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	fs := store.NewFileStore(path)
	ctx := context.Background()

	if err := fs.Save(ctx, testSnap); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after first Save: %v", err)
	}

	if err := fs.Save(ctx, testSnap); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after second Save: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Errorf("file content differs between two identical saves:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
