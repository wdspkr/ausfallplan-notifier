package ausfallplan

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestSimpleTable(t *testing.T) {
	html, err := os.ReadFile("simple_table.html")
	if err != nil {
		panic(err)
	}
	expectedEntries := []Entry{
		{Day: time.Date(2023, 02, 06, 0, 0, 0, 0, time.UTC), Hour: "6. Stunde", Class: "JüL 3, JüL 7", Information: "Englisch"},
		{Day: time.Date(2023, 02, 06, 0, 0, 0, 0, time.UTC), Hour: "7./ 8. Stunde", Class: "Geige"},
		{Day: time.Date(2023, 02, 07, 0, 0, 0, 0, time.UTC), Hour: "7. Stunde", Class: "6 a, b, c", Information: "Saxophon"},
		{Day: time.Date(2023, 02, 07, 0, 0, 0, 0, time.UTC), Hour: "7. Stunde", Class: "4a, b", Information: "Querflöte"},
	}

	entries := parse(html)

	if !reflect.DeepEqual(entries, expectedEntries) {
		t.Errorf("No match!\n Expected: %v\n Got: %v", expectedEntries, entries)
	}
}

func TestEmptyTable(t *testing.T) {
	html, err := os.ReadFile("empty_table.html")
	if err != nil {
		panic(err)
	}
	expectedEntries := []Entry{}

	entries := parse(html)
	if !reflect.DeepEqual(entries, expectedEntries) {
		t.Errorf("No match!\n Expected: %v\n Got: %v", expectedEntries, entries)
	}
}
