package main

import (
	"errors"
	"fmt"

	sugoiconfig "github.com/blamoo/sugoi/internal/config"
)

// Temporary aliases while the code moves into internal packages.
type SugoiConfig = sugoiconfig.Config
type SlideshowPreset = sugoiconfig.SlideshowPreset

var config SugoiConfig

const RANDOM_SAMPLE_SIZE = 1000

func InitializeConfig() error {
	c, err := sugoiconfig.Load(configPath)

	var keyErr sugoiconfig.SessionKeyError
	if errors.As(err, &keyErr) {
		fmt.Println(keyErr.Error())
		fmt.Println("Like this:")
		fmt.Println(keyErr.Suggested)
		return fmt.Errorf("update your configuration file with a valid value, then run sugoi again")
	}
	if err != nil {
		return err
	}

	config = *c
	return nil
}
