package ausfallplan

import (
	"fmt"
	"net/http"
	"os"
)

func fetch_page() []byte {
	resp, err := http.Get(os.Getenv("AUSFALL_URL"))

	if err != nil {
		fmt.Println("could not fetch page:", err)
		os.Exit(1)
	}

	bs := make([]byte, 100000)
	resp.Body.Read(bs)

	return bs
}

func load_file() []byte {
	html, err := os.ReadFile("ausfallplan.html")
	if err != nil {
		panic(err)
	}
	return html
}
