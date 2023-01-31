package ausfallplan

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestSimpleTable(t *testing.T) {
	html, err := os.ReadFile("simple_ausfallplan.html")
	if err != nil {
		panic(err)
	}
	expectedEntries := []Entry{
		{day: time.Date(2023, 02, 06, 0, 0, 0, 0, time.UTC), hour: "6. Stunde", class: "JüL 3, JüL 7", information: "Englisch"},
		{day: time.Date(2023, 02, 06, 0, 0, 0, 0, time.UTC), hour: "7./ 8. Stunde", class: "Geige"},
		{day: time.Date(2023, 02, 07, 0, 0, 0, 0, time.UTC), hour: "7. Stunde", class: "6 a, b, c", information: "Saxophon"},
		{day: time.Date(2023, 02, 07, 0, 0, 0, 0, time.UTC), hour: "7. Stunde", class: "4a, b", information: "Querflöte"},
	}

	entries := parse(html)

	if !reflect.DeepEqual(entries, expectedEntries) {
		t.Errorf("That did not work...")
	}
}

func TestEmptyTable(t *testing.T) {

}
