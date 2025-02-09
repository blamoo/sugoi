package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path"
)

var files map[string]string = map[string]string{
	"static/js/jquery.min.js":                  "https://cdnjs.cloudflare.com/ajax/libs/jquery/3.7.1/jquery.min.js",
	"static/js/bootstrap.min.js":               "https://cdnjs.cloudflare.com/ajax/libs/bootstrap/5.3.3/js/bootstrap.bundle.min.js",
	"static/css/bootswatch.min.css":            "https://cdnjs.cloudflare.com/ajax/libs/bootswatch/5.3.3/darkly/bootstrap.min.css",
	"static/css/font-awesome.min.css":          "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/css/all.min.css",
	"static/webfonts/fa-brands-400.woff2":      "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-brands-400.woff2",
	"static/webfonts/fa-brands-400.ttf":        "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-brands-400.ttf",
	"static/webfonts/fa-regular-400.woff2":     "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-regular-400.woff2",
	"static/webfonts/fa-regular-400.ttf":       "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-regular-400.ttf",
	"static/webfonts/fa-solid-900.woff2":       "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-solid-900.woff2",
	"static/webfonts/fa-solid-900.ttf":         "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-solid-900.ttf",
	"static/webfonts/fa-v4compatibility.woff2": "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-v4compatibility.woff2",
	"static/webfonts/fa-v4compatibility.ttf":   "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-v4compatibility.ttf",
}

func main() {
	for fn, url := range files {
		res, err := http.Get(url)

		if err != nil {
			log.Fatal(err)
		}

		defer res.Body.Close()

		dir := path.Dir(fn)

		os.MkdirAll(dir, 0775)

		dest, err := os.Create(fn)

		if err != nil {
			log.Fatal(err)
		}

		defer dest.Close()

		io.Copy(dest, res.Body)
		log.Printf("%s => %s\n", url, fn)
	}
}
