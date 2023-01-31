package ausfallplan

import (
	"fmt"
	"time"
)

type Entry struct {
	day         time.Time
	hour        string
	class       string
	information string
}

func GetEntriesFor() {
	html := load_file()
	entries := parse(html)
	// Filter entries for class

	for _, s := range entries {
		fmt.Print(s.day)
		fmt.Print(s.hour)
		fmt.Print(s.class)
		fmt.Print(s.information)
		fmt.Print("\n")
	}
}
