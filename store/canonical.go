package store

import (
	"sort"

	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
)

// canonicalize returns a new Snapshot with Entries sorted by Day, Hour, Class,
// Information and Infos sorted by Text. It never mutates the input.
func canonicalize(snap ausfallplan.Snapshot) ausfallplan.Snapshot {
	entries := make([]ausfallplan.Entry, len(snap.Entries))
	copy(entries, snap.Entries)
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if !a.Day.Equal(b.Day) {
			return a.Day.Before(b.Day)
		}
		if a.Hour != b.Hour {
			return a.Hour < b.Hour
		}
		if a.Class != b.Class {
			return a.Class < b.Class
		}
		return a.Information < b.Information
	})

	infos := make([]ausfallplan.Info, len(snap.Infos))
	copy(infos, snap.Infos)
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Text < infos[j].Text
	})

	return ausfallplan.Snapshot{Entries: entries, Infos: infos}
}
