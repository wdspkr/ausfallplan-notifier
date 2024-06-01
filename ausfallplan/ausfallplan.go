package ausfallplan

import (
	"time"
)

type Entry struct {
	Day         time.Time
	Hour        string
	Class       string
	Information string
}

func GetEntriesFor(level string, class string) []Entry {
	// html := fetch_page()
	html := load_file()
	entries := parse(html)
	filtered_entries := FilterEntries(entries, level, class)

	return filtered_entries
}
func GetAllEntries() []Entry {
	// html := fetch_page()
	html := load_file()
	entries := parse(html)

	return entries
}
