package ausfallplan

import (
	"reflect"
	"testing"
	"time"
)

func TestFilter(t *testing.T) {
	entries := []Entry{
		{Day: time.Date(2023, 02, 06, 0, 0, 0, 0, time.UTC), Hour: "6. Stunde", Class: "JüL 3, JüL 7", Information: "Englisch"},
		{Day: time.Date(2023, 02, 06, 0, 0, 0, 0, time.UTC), Hour: "7./ 8. Stunde", Class: "Geige"},
		{Day: time.Date(2023, 02, 07, 0, 0, 0, 0, time.UTC), Hour: "7. Stunde", Class: "jül3", Information: "Saxophon"},
		{Day: time.Date(2023, 02, 07, 0, 0, 0, 0, time.UTC), Hour: "7. Stunde", Class: "JüL 4, 6, 3", Information: "Querflöte"},
	}

	expectedEntries := []Entry{
		{Day: time.Date(2023, 02, 06, 0, 0, 0, 0, time.UTC), Hour: "6. Stunde", Class: "JüL 3, JüL 7", Information: "Englisch"},
		{Day: time.Date(2023, 02, 07, 0, 0, 0, 0, time.UTC), Hour: "7. Stunde", Class: "jül3", Information: "Saxophon"},
		{Day: time.Date(2023, 02, 07, 0, 0, 0, 0, time.UTC), Hour: "7. Stunde", Class: "JüL 4, 6, 3", Information: "Querflöte"},
	}

	filteredEntries := FilterEntries(entries, "Jül", "3")

	if !reflect.DeepEqual(filteredEntries, expectedEntries) {
		t.Errorf("No match!\n Expected: %v\n Got: %v", expectedEntries, filteredEntries)
	}
}
