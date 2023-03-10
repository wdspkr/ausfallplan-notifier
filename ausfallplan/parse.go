package ausfallplan

import (
	"regexp"
	"time"
)

func parse(html []byte) []Entry {
	table := extractTable(html)
	rows := splitRows(table)

	return entries(rows[1:])
}

func entries(rows [][][]byte) []Entry {
	var lastRowDate time.Time
	entries := []Entry{}
	rgx, _ := regexp.Compile(`(?s)<td.*?>(.*?)</td>`)

	for _, tr := range rows {
		raw := rgx.FindAllSubmatch(tr[1], 4)
		entry := Entry{
			Day:         dateTime(raw[0][1], &lastRowDate),
			Hour:        string(raw[1][1]),
			Class:       string(raw[2][1]),
			Information: string(raw[3][1]),
		}

		if entry.Hour == "" {
			continue
		}

		entries = append(entries, entry)
	}
	return entries
}

func dateTime(raw []byte, lastDate *time.Time) time.Time {
	rgx, _ := regexp.Compile(`\d{2}\.\d{2}\.\d{4}`)
	dateBytes := rgx.Find(raw)

	if len(dateBytes) == 0 {
		return *lastDate
	}

	date, err := time.Parse("02.01.2006", string(dateBytes))
	if err != nil {
		panic(err)
	}

	*lastDate = date
	return date
}

func splitRows(table []byte) [][][]byte {
	rgx, _ := regexp.Compile(`(?s)<tr.*?>(.*?)</tr>`)
	rows := rgx.FindAllSubmatch(table, -1)
	return rows
}

func extractTable(html []byte) []byte {
	rgx, _ := regexp.Compile(`(?s)<table id="tablepress-1".*?/table>`)
	tab := rgx.Find(html)
	return tab
}
