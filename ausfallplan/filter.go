package ausfallplan

import (
	"fmt"
	"regexp"
)

func FilterEntries(entries []Entry, level string, class string) []Entry {
	filteredEntries := []Entry{}

	rgxStr := fmt.Sprint("(?i)", level, ".*", class)
	rgx := regexp.MustCompile(rgxStr)

	for _, entry := range entries {
		if rgx.MatchString(entry.Class) {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	return filteredEntries
}
