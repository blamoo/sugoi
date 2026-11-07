// Command static downloads the vendored frontend libraries into static/
// (run by `go generate` from the repository root).
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
)

var files map[string]string = map[string]string{
	"static/js/jquery.min.js":                  "https://cdnjs.cloudflare.com/ajax/libs/jquery/4.0.0/jquery.min.js",
	"static/js/bootstrap.min.js":               "https://cdnjs.cloudflare.com/ajax/libs/bootstrap/5.3.8/js/bootstrap.bundle.min.js",
	"static/css/bootswatch.min.css":            "https://cdnjs.cloudflare.com/ajax/libs/bootswatch/5.3.8/darkly/bootstrap.min.css",
	"static/css/font-awesome.min.css":          "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/7.3.1/css/all.min.css",
	"static/webfonts/fa-brands-400.woff2":      "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/7.3.1/webfonts/fa-brands-400.woff2",
	"static/webfonts/fa-regular-400.woff2":     "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/7.3.1/webfonts/fa-regular-400.woff2",
	"static/webfonts/fa-solid-900.woff2":       "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/7.3.1/webfonts/fa-solid-900.woff2",
	"static/webfonts/fa-v4compatibility.woff2": "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/7.3.1/webfonts/fa-v4compatibility.woff2",
}

func main() {
	for fn, url := range files {
		err := download(url, fn)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("%s => %s\n", url, fn)
	}
}

// download saves url to fn, failing on any non-200 response instead of
// writing an error page in place of the file.
func download(url string, fn string) error {
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, res.Status)
	}

	err = os.MkdirAll(path.Dir(fn), 0775)
	if err != nil {
		return err
	}

	dest, err := os.Create(fn)
	if err != nil {
		return err
	}

	_, err = io.Copy(dest, res.Body)
	if err != nil {
		dest.Close()
		return err
	}

	return dest.Close()
}
