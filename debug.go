package main

import (
	"fmt"
)

func debugPrintf(format string, a ...any) {
	if config.Debug {
		fmt.Printf(format, a...)
		return
	}
}
