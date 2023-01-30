package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	dat, err := os.ReadFile("ausfallplan.html")
	check(err)

	tabregx, _ := regexp.Compile(`(?s)<table id="tablepress-1".*?/table>`)
	tab := tabregx.Find(dat)

	trregx, _ := regexp.Compile(`(?s)<tr.*?>(.*?)</tr>`)
	trs := trregx.FindAllSubmatch(tab, -1)

	tdregx, _ := regexp.Compile(`(?s)<td.*?>(.*?)</td>`)

	entries := make([]string, len(trs))
	for i, tr := range trs {
		data := tdregx.FindAllSubmatch(tr[1], 4)

		entry := make([]string, 4)
		for i, x := range data {
			entry[i] = string(x[1])
		}

		entries[i] = strings.Join(entry, ", ")
	}

	for _, s := range entries {
		fmt.Print(s)
		fmt.Print("\n")
	}
}
