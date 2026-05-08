package diff_test

import (
	"testing"
	"time"

	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
	"github.com/wdspkr/ausfallplan-notifier/diff"
)

var (
	day1 = time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	day2 = time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)

	entry1 = ausfallplan.Entry{Day: day1, Hour: "1./2. Stunde", Class: "3d", Information: "Ausfall"}
	entry2 = ausfallplan.Entry{Day: day1, Hour: "3. Stunde", Class: "6b", Information: "Vertretung"}
	entry3 = ausfallplan.Entry{Day: day2, Hour: "0./1. Stunde", Class: "4c", Information: ""}

	info1 = ausfallplan.Info{Text: "Morgen kein Sport"}
	info2 = ausfallplan.Info{Text: "Schulausflug 4c"}
)

func TestCompute_BothEmpty(t *testing.T) {
	added := diff.Compute(ausfallplan.Snapshot{}, ausfallplan.Snapshot{})
	if len(added.Entries) != 0 {
		t.Errorf("expected 0 added entries, got %d", len(added.Entries))
	}
	if len(added.Infos) != 0 {
		t.Errorf("expected 0 added infos, got %d", len(added.Infos))
	}
}

func TestCompute_EmptyPrev_AllNext(t *testing.T) {
	prev := ausfallplan.Snapshot{}
	next := ausfallplan.Snapshot{
		Entries: []ausfallplan.Entry{entry1, entry2},
		Infos:   []ausfallplan.Info{info1},
	}

	added := diff.Compute(prev, next)

	if len(added.Entries) != 2 {
		t.Errorf("expected 2 added entries, got %d", len(added.Entries))
	}
	if added.Entries[0] != entry1 {
		t.Errorf("expected first entry to be entry1, got %+v", added.Entries[0])
	}
	if added.Entries[1] != entry2 {
		t.Errorf("expected second entry to be entry2, got %+v", added.Entries[1])
	}
	if len(added.Infos) != 1 || added.Infos[0] != info1 {
		t.Errorf("expected info1 added, got %+v", added.Infos)
	}
}

func TestCompute_RemovedEntries_NotReturned(t *testing.T) {
	prev := ausfallplan.Snapshot{
		Entries: []ausfallplan.Entry{entry1, entry2},
		Infos:   []ausfallplan.Info{info1},
	}
	next := ausfallplan.Snapshot{}

	added := diff.Compute(prev, next)

	if len(added.Entries) != 0 {
		t.Errorf("expected 0 added entries (removals ignored), got %d", len(added.Entries))
	}
	if len(added.Infos) != 0 {
		t.Errorf("expected 0 added infos (removals ignored), got %d", len(added.Infos))
	}
}

func TestCompute_SameSnapshot_NoAdditions(t *testing.T) {
	snap := ausfallplan.Snapshot{
		Entries: []ausfallplan.Entry{entry1, entry2},
		Infos:   []ausfallplan.Info{info1},
	}

	added := diff.Compute(snap, snap)

	if len(added.Entries) != 0 {
		t.Errorf("expected 0 added entries, got %d", len(added.Entries))
	}
	if len(added.Infos) != 0 {
		t.Errorf("expected 0 added infos, got %d", len(added.Infos))
	}
}

func TestCompute_OneNewEntry(t *testing.T) {
	prev := ausfallplan.Snapshot{
		Entries: []ausfallplan.Entry{entry1, entry2},
	}
	next := ausfallplan.Snapshot{
		Entries: []ausfallplan.Entry{entry1, entry2, entry3},
	}

	added := diff.Compute(prev, next)

	if len(added.Entries) != 1 {
		t.Errorf("expected 1 added entry, got %d", len(added.Entries))
	}
	if added.Entries[0] != entry3 {
		t.Errorf("expected entry3, got %+v", added.Entries[0])
	}
}

func TestCompute_ReorderedEntries_NoAdditions(t *testing.T) {
	prev := ausfallplan.Snapshot{
		Entries: []ausfallplan.Entry{entry1, entry2, entry3},
	}
	// Same entries in different order.
	next := ausfallplan.Snapshot{
		Entries: []ausfallplan.Entry{entry3, entry1, entry2},
	}

	added := diff.Compute(prev, next)

	if len(added.Entries) != 0 {
		t.Errorf("expected 0 added entries (reorder, not addition), got %d", len(added.Entries))
	}
}

func TestCompute_NewInfo(t *testing.T) {
	prev := ausfallplan.Snapshot{
		Infos: []ausfallplan.Info{info1},
	}
	next := ausfallplan.Snapshot{
		Infos: []ausfallplan.Info{info1, info2},
	}

	added := diff.Compute(prev, next)

	if len(added.Entries) != 0 {
		t.Errorf("expected 0 added entries, got %d", len(added.Entries))
	}
	if len(added.Infos) != 1 || added.Infos[0] != info2 {
		t.Errorf("expected info2 added, got %+v", added.Infos)
	}
}
