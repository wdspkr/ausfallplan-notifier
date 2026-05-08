package ausfallplan

import (
	"os"
	"testing"
	"time"
)

func TestSimpleTable(t *testing.T) {
	html, err := os.ReadFile("testdata/simple_table.html")
	if err != nil {
		t.Fatal(err)
	}
	expected := []Entry{
		{Day: time.Date(2023, 02, 06, 0, 0, 0, 0, time.UTC), Hour: "6. Stunde", Class: "3. Klassen", Information: "Englisch"},
		{Day: time.Date(2023, 02, 06, 0, 0, 0, 0, time.UTC), Hour: "7./ 8. Stunde", Class: "Geige", Information: ""},
		{Day: time.Date(2023, 02, 07, 0, 0, 0, 0, time.UTC), Hour: "7. Stunde", Class: "6 a, b, c", Information: "Saxophon"},
		{Day: time.Date(2023, 02, 07, 0, 0, 0, 0, time.UTC), Hour: "7. Stunde", Class: "4a, b", Information: "Querflöte"},
	}

	snap, err := Parse(html)
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	if len(snap.Entries) != len(expected) {
		t.Fatalf("expected %d entries, got %d: %v", len(expected), len(snap.Entries), snap.Entries)
	}
	for i, e := range expected {
		if snap.Entries[i] != e {
			t.Errorf("entry[%d]: expected %+v, got %+v", i, e, snap.Entries[i])
		}
	}
	if len(snap.Infos) != 0 {
		t.Errorf("expected 0 Infos, got %d", len(snap.Infos))
	}
}

func TestEmptyTable(t *testing.T) {
	html, err := os.ReadFile("testdata/empty_table.html")
	if err != nil {
		t.Fatal(err)
	}

	snap, err := Parse(html)
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	if len(snap.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d: %v", len(snap.Entries), snap.Entries)
	}
	if len(snap.Infos) != 0 {
		t.Errorf("expected 0 Infos, got %d", len(snap.Infos))
	}
}

func TestInfoTableSingleRow(t *testing.T) {
	html, err := os.ReadFile("testdata/info_single.html")
	if err != nil {
		t.Fatal(err)
	}

	snap, err := Parse(html)
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	if len(snap.Infos) != 1 {
		t.Fatalf("expected 1 Info, got %d: %v", len(snap.Infos), snap.Infos)
	}
	if snap.Infos[0].Text != "Kein Sportunterricht am Mittwoch." {
		t.Errorf("unexpected Info text: %q", snap.Infos[0].Text)
	}
}

func TestInfoTableMultipleRows(t *testing.T) {
	html, err := os.ReadFile("testdata/info_multi.html")
	if err != nil {
		t.Fatal(err)
	}

	snap, err := Parse(html)
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	expected := []Info{
		{Text: "Kein Sportunterricht am Mittwoch."},
		{Text: "Elternabend am Donnerstag um 19 Uhr."},
		{Text: "Schulausflug am Freitag für Klasse 3d."},
	}
	if len(snap.Infos) != len(expected) {
		t.Fatalf("expected %d Infos, got %d: %v", len(expected), len(snap.Infos), snap.Infos)
	}
	for i, inf := range expected {
		if snap.Infos[i] != inf {
			t.Errorf("Info[%d]: expected %+v, got %+v", i, inf, snap.Infos[i])
		}
	}
}

func TestInfoTableEmpty(t *testing.T) {
	html, err := os.ReadFile("testdata/info_empty.html")
	if err != nil {
		t.Fatal(err)
	}

	snap, err := Parse(html)
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	if len(snap.Infos) != 0 {
		t.Errorf("expected 0 Infos for empty info table, got %d: %v", len(snap.Infos), snap.Infos)
	}
}

func TestLiveSnapshot(t *testing.T) {
	html, err := os.ReadFile("testdata/live_snapshot.html")
	if err != nil {
		t.Fatal(err)
	}

	snap, err := Parse(html)
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	if len(snap.Entries) == 0 {
		t.Error("expected at least one entry from live snapshot")
	}
	for i, e := range snap.Entries {
		if e.Day.IsZero() {
			t.Errorf("entry[%d] has zero Day (header row leaked through?): %+v", i, e)
		}
	}
	// live_snapshot's tablepress-2 has only a <br /> row → Infos should be empty
	if len(snap.Infos) != 0 {
		t.Errorf("expected 0 Infos from live snapshot, got %d", len(snap.Infos))
	}
}

func TestHeaderInTbody(t *testing.T) {
	html, err := os.ReadFile("testdata/header_in_tbody.html")
	if err != nil {
		t.Fatal(err)
	}

	snap, err := Parse(html)
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	expected := []Entry{
		{Day: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC), Hour: "0./1. Stunde", Class: "4c", Information: ""},
		{Day: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC), Hour: "1. Stunde", Class: "3b", Information: "Ukulele"},
	}
	if len(snap.Entries) != len(expected) {
		t.Fatalf("expected %d entries (header should be skipped), got %d: %v", len(expected), len(snap.Entries), snap.Entries)
	}
	for i, e := range expected {
		if snap.Entries[i] != e {
			t.Errorf("entry[%d]: expected %+v, got %+v", i, e, snap.Entries[i])
		}
	}
}

func TestParseMissingTablepress1(t *testing.T) {
	html := []byte("<html><body></body></html>")
	_, err := Parse(html)
	if err == nil {
		t.Error("expected an error when tablepress-1 is absent, got nil")
	}
}
