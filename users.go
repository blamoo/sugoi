package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/blamoo/sugoi/internal/config"
	"github.com/blamoo/sugoi/internal/web"
)

// ManageUsers adds a user or changes a password (-u), then offers to rewrite
// the config file.
func ManageUsers(cfg *config.Config, configPath string) {
	var username string
	fmt.Print("Username: ")
	fmt.Scanln(&username)
	fmt.Print("Password: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Print("**************")
	fmt.Println()

	if err != nil {
		fmt.Println(err)
		os.Exit(4)
	}

	if len(password) < 6 {
		fmt.Println("Password should have at least 6 characters")
		os.Exit(3)
	}

	hashedPassword, err := web.HashPassword(string(password))

	if err != nil {
		fmt.Println(err)
		os.Exit(7)
	}

	if cfg.Users == nil {
		cfg.Users = make(map[string]string)
	}
	_, exists := cfg.Users[username]

	cfg.Users[username] = hashedPassword

	newFile, err := cfg.Export()
	if err != nil {
		fmt.Println(err)
		os.Exit(10)
	}

	fmt.Println("Updated config file:")
	fmt.Println(newFile)

	var overwrite bool
	for {
		var decision string
		fmt.Printf("Overwrite %s with this? [Y/n] ", configPath)
		fmt.Scanln(&decision)

		decision = strings.ToLower(decision)

		if decision == "" || decision == "y" || decision == "yes" {
			overwrite = true
			break
		}

		if decision == "n" || decision == "no" {
			overwrite = false
			break
		}
	}

	if overwrite {
		err = cfg.Save(configPath)

		if err != nil {
			fmt.Println(err)
			os.Exit(9)
		}

		if exists {
			fmt.Printf("Password for user %s changed and saved\n", username)
		} else {
			fmt.Printf("User %s created and saved\n", username)
		}
	}
}
