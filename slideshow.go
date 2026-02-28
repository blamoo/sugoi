package main

import "fmt"

type SlideshowPreset struct {
	Seconds int
	Default bool
}

func (c SlideshowPreset) Label() string {
	if c.Seconds == 1 {
		return "1 second"
	}

	return fmt.Sprintf("%d seconds", c.Seconds)
}
