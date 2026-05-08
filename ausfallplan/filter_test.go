package ausfallplan

import (
	"testing"
	"time"
)

// makeEntry is a helper that builds an Entry with the given Class field.
func makeEntry(class string) Entry {
	return Entry{
		Day:         time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
		Hour:        "1. Stunde",
		Class:       class,
		Information: "Test",
	}
}

// userBlacklist is the blacklist for the user's children (3d, 6b kept; all others blacklisted).
// Mirrors config.json. 4e is included because Stechlinsee has an unusual fifth 4th-grade group.
var userBlacklist = []string{
	"1a", "1b", "1c", "1d",
	"2a", "2b", "2c", "2d",
	"3a", "3b", "3c",
	"4a", "4b", "4c", "4d", "4e",
	"5a", "5b", "5c", "5d",
	"6a", "6c", "6d",
}

// TestFilter_TableCases covers every row in the spec table.
func TestFilter_TableCases(t *testing.T) {
	cases := []struct {
		name    string
		class   string
		wantLen int // 1 = kept, 0 = dropped
	}{
		{"Pure3d_Kept", "3d", 1},
		{"Pure1a_Dropped", "1a", 0},
		{"1a_3d_Kept", "1a, 3d", 1},
		{"1a_2b_Dropped", "1a, 2b", 0},
		{"3DotKlassen_Kept", "3. Klassen", 1},
		{"alleKlassen_Kept", "alle Klassen", 1},
		{"Geige_Kept", "Geige", 1},
		{"JüL3_JüL7_Kept", "JüL 3, JüL 7", 1},
		{"SlashFive6_Kept", "5/6", 1},
		{"Empty_Kept", "", 1},
		{"3DSpace_Uppercase_Kept", "3 D", 1},
		{"YearCarry_6abc_Kept", "6 a, b, c", 1},    // 6b not in blacklist
		{"YearCarry_4ab_Dropped", "4a, b", 0},       // both 4a and 4b are blacklisted
		{"6a_6c_6d_Dropped", "6a, 6c, 6d", 0},
		{"YearCarry_6acd_Dropped", "6 a, c, d", 0},  // 6a,6c,6d all blacklisted
		{"BAlone_Kept", "b", 1},                     // no prior year, unrecognized
		{"3aEnglisch_Kept", "3a Englisch", 1},        // extra text in fragment, unrecognized
		{"Pure4e_Dropped", "4e", 0},                 // letter outside a-d but blacklisted
		{"4d_4e_Dropped", "4d, 4e", 0},              // both blacklisted (Stechlinsee real case)
		{"YearCarry_4de_Dropped", "4 d, e", 0},      // year-carry with letter outside a-d
		{"5fAlone_Kept", "5f", 1},                   // recognized token but not in blacklist
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := makeEntry(tc.class)
			result := Filter([]Entry{entry}, userBlacklist)
			if len(result) != tc.wantLen {
				t.Errorf("Filter(%q): got %d entries, want %d", tc.class, len(result), tc.wantLen)
			}
		})
	}
}

// TestFilter_YearCarry_4ab asserts that "4a, b" tokenizes to [4a, 4b] and gets dropped.
func TestFilter_YearCarry_4ab(t *testing.T) {
	entry := makeEntry("4a, b")
	result := Filter([]Entry{entry}, userBlacklist)
	if len(result) != 0 {
		t.Errorf("Filter(\"4a, b\"): expected entry to be dropped, but it was kept")
	}
}

// TestFilter_ExtractClasses_UnrecognizedFragment asserts that a purely unrecognized
// fragment causes hasUnrecognized=true, which results in KEEP.
func TestFilter_ExtractClasses_Unrecognized(t *testing.T) {
	// "foo" is not a valid class token — hasUnrecognized must be true → KEEP.
	entry := makeEntry("foo")
	result := Filter([]Entry{entry}, userBlacklist)
	if len(result) != 1 {
		t.Errorf("Filter(\"foo\"): expected entry to be kept (hasUnrecognized=true), but it was dropped")
	}
}

// TestFilter_EmptyBlacklist_KeepsAll asserts that an empty blacklist keeps everything.
func TestFilter_EmptyBlacklist_KeepsAll(t *testing.T) {
	entries := []Entry{
		makeEntry("1a"),
		makeEntry("2b"),
		makeEntry("3d"),
	}
	result := Filter(entries, []string{})
	if len(result) != 3 {
		t.Errorf("Filter with empty blacklist: got %d entries, want 3", len(result))
	}
}

// TestFilter_NilBlacklist_KeepsAll asserts that a nil blacklist keeps everything.
func TestFilter_NilBlacklist_KeepsAll(t *testing.T) {
	entries := []Entry{makeEntry("1a")}
	result := Filter(entries, nil)
	if len(result) != 1 {
		t.Errorf("Filter with nil blacklist: got %d entries, want 1", len(result))
	}
}

// TestFilter_MultipleEntries_MixedOutcomes asserts entries are independently filtered.
func TestFilter_MultipleEntries_MixedOutcomes(t *testing.T) {
	entries := []Entry{
		makeEntry("1a"),   // dropped
		makeEntry("3d"),   // kept
		makeEntry("2b"),   // dropped
		makeEntry("alle Klassen"), // kept (unrecognized)
	}
	result := Filter(entries, userBlacklist)
	if len(result) != 2 {
		t.Errorf("Filter mixed entries: got %d entries, want 2", len(result))
	}
	if result[0].Class != "3d" {
		t.Errorf("Filter mixed entries: result[0].Class = %q, want %q", result[0].Class, "3d")
	}
	if result[1].Class != "alle Klassen" {
		t.Errorf("Filter mixed entries: result[1].Class = %q, want %q", result[1].Class, "alle Klassen")
	}
}

// TestFilter_BlacklistNormalization asserts that blacklist entries with spaces/uppercase
// are normalized correctly.
func TestFilter_BlacklistNormalization(t *testing.T) {
	// Blacklist has "3 D" (with space, uppercase) — after normalization it becomes "3d".
	entry := makeEntry("3d")
	result := Filter([]Entry{entry}, []string{"3 D"})
	if len(result) != 0 {
		t.Errorf("Filter with normalized blacklist entry \"3 D\": expected 3d to be dropped, but it was kept")
	}
}
