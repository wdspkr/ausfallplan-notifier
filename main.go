package main

import (
	"fmt"
	"os"

	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
)

func main() {
	entries := ausfallplan.GetEntriesFor(os.Getenv("LEVEL"), os.Getenv("CLASS"))

	for _, s := range entries {
		fmt.Print(s.Day)
		fmt.Print(s.Hour)
		fmt.Print(s.Class)
		fmt.Print(s.Information)
		fmt.Print("\n")
	}
}
