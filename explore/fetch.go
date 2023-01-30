package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	resp, err := http.Get("https://stechlinsee-grundschule.de/ausfall-plan/")

	if err != nil {
		fmt.Println("could not fetch page:", err)
		os.Exit(1)
	}

	bs := make([]byte, 99999)
	resp.Body.Read(bs)

	fmt.Println(string(bs))

	resp.Body.Read(bs)

	fmt.Println(string(bs))

	// io.Copy()
	// bufio.NewReadWriter()

}
