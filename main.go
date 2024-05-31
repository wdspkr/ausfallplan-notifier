package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/wdspkr/ausfallplan-notifier/ausfallplan"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err, "Error loading .env file")
	}

	entries := ausfallplan.GetEntriesFor(os.Getenv("LEVEL"), os.Getenv("CLASS"))

	for _, s := range entries {
		formattedDay := s.Day.Format("02.01.2006")
		fmt.Printf("%s %s %s %s\n", formattedDay, s.Hour, s.Class, s.Information)
	}
}
