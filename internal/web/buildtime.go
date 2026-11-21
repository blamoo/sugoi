package web

import (
	"os"
	"time"
)

var buildStr string
var buildTime time.Time

func init() {
	fileStat, err := os.Stat(os.Args[0])

	if err != nil {
		buildStr = "-"
		return
	}

	buildTime = fileStat.ModTime()
	buildStr = buildTime.Format("2006-01-02 15:04")
}

func BuildDiff() string {
	return DurationFormat(time.Since(buildTime))
}
