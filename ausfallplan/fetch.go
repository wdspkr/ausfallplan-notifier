package ausfallplan

import (
	"io"
	"log"
	"net/http"
	"os"
)

func fetch_page() []byte {
	res, err := http.Get(os.Getenv("AUSFALL_URL"))

	if err != nil {
		log.Fatal(err)
	}
	content, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	return content
}

func load_file() []byte {
	html, err := os.ReadFile("ausfallplan.html")
	if err != nil {
		panic(err)
	}
	return html
}
