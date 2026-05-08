package ausfallplan

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Entry represents one row from tablepress-1 (Ausfallplan).
type Entry struct {
	Day         time.Time
	Hour        string
	Class       string
	Information string
}

// Info represents one row from tablepress-2 (Aktuelle Informationen).
type Info struct {
	Text string
}

// Snapshot holds all data parsed from a single page fetch.
type Snapshot struct {
	Entries []Entry
	Infos   []Info
}

var dateRe = regexp.MustCompile(`\d{2}\.\d{2}\.\d{4}`)

// Parse parses both tablepress-1 (Ausfall) and tablepress-2 (Aktuelle Informationen).
// It returns a Snapshot. If tablepress-1 is missing, it returns an error.
// tablepress-2 is allowed to be absent — Infos is just empty.
// Empty tables (no data rows) are not errors.
func Parse(html []byte) (Snapshot, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return Snapshot{}, fmt.Errorf("ausfallplan: parse HTML: %w", err)
	}

	// tablepress-1 is required.
	tp1 := doc.Find("#tablepress-1")
	if tp1.Length() == 0 {
		return Snapshot{}, fmt.Errorf("ausfallplan: tablepress-1 not found")
	}

	entries, err := parseEntries(tp1)
	if err != nil {
		return Snapshot{}, err
	}

	// tablepress-2 is optional.
	infos := parseInfos(doc.Find("#tablepress-2"))

	return Snapshot{Entries: entries, Infos: infos}, nil
}

func parseEntries(table *goquery.Selection) ([]Entry, error) {
	var entries []Entry
	var lastDate time.Time
	var parseErr error

	table.Find("tbody tr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		cells := row.Find("td")
		if cells.Length() < 4 {
			return true // skip malformed rows
		}

		col := func(i int) string {
			return strings.TrimSpace(cells.Eq(i).Text())
		}

		dayText := col(0)
		hour := col(1)
		class := col(2)
		information := col(3)

		// Parse or carry date.
		var day time.Time
		if dayText == "" {
			day = lastDate
		} else {
			dateStr := dateRe.FindString(dayText)
			if dateStr == "" {
				// No date pattern found — carry last date.
				day = lastDate
			} else {
				parsed, err := time.ParseInLocation("02.01.2006", dateStr, time.UTC)
				if err != nil {
					parseErr = fmt.Errorf("ausfallplan: parse date %q: %w", dateStr, err)
					return false // stop iteration
				}
				day = parsed
				lastDate = day
			}
		}

		// Skip rows where Hour is empty (existing behavior).
		if hour == "" {
			return true
		}

		// Skip rows that have no date context yet — covers header rows
		// placed inside <tbody> with cells like "Datum", "Stunde(n)" etc.
		// The live TablePress output has no <thead>; the first <tr> is a header.
		if day.IsZero() {
			return true
		}

		entries = append(entries, Entry{
			Day:         day,
			Hour:        hour,
			Class:       class,
			Information: information,
		})
		return true
	})

	if parseErr != nil {
		return nil, parseErr
	}

	if entries == nil {
		entries = []Entry{}
	}
	return entries, nil
}

func parseInfos(table *goquery.Selection) []Info {
	var infos []Info

	table.Find("tbody tr").Each(func(_ int, row *goquery.Selection) {
		text := strings.TrimSpace(row.Find("td").First().Text())
		if text == "" {
			return
		}
		infos = append(infos, Info{Text: text})
	})

	return infos
}
