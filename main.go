package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"

	"github.com/blevesearch/bleve/v2"
	"github.com/gorilla/mux"

	"golang.org/x/term"
)

//go:generate go run cmd/static/main.go

var configPath string
var filePointers FilePointerList
var bleveIndex bleve.Index
var httproot string
var reindexJob ReindexJob

func main() {
	var err error

	flag.StringVar(&configPath, "c", "./config/sugoi.json", "Path to the configuration file. Default: ./config/sugoi.json")
	user := flag.Bool("u", false, "Adds a new user or changes de password of an existing user on your config file.")
	flag.Parse()

	err = InitializeConfig()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if user != nil && *user {
		ManageUsers()
		return
	}

	InitializeSession()

	err = InitializeFilePointers()
	if err != nil {
		fmt.Println(err)
		os.Exit(5)
	}

	err = InitializeBleve()
	if err != nil {
		fmt.Println(err)
		os.Exit(8)
	}

	InitializeOrder()

	router := mux.NewRouter()

	semaphore := make(chan struct{}, config.MaxConcurrency)

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			semaphore <- struct{}{}
			defer func() {
				<-semaphore
			}()

			next.ServeHTTP(w, r)
		})
	})

	Routes(router)

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	fmt.Println("uwu")

	if config.ServerPort == 80 {
		httproot = fmt.Sprintf("http://%s/", config.ServerHost)
	} else {
		httproot = fmt.Sprintf("http://%s:%d/\n", config.ServerHost, config.ServerPort)
	}

	fmt.Printf("Listening on %s\n", httproot)

	err = http.ListenAndServe(fmt.Sprintf("%s:%d", config.ServerHost, config.ServerPort), router)
	if err != nil {
		fmt.Println(err)
		os.Exit(4)
	}
	fmt.Println("owo")
}

func ManageUsers() {
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

	hashedPassword, err := HashPassword(string(password))

	if err != nil {
		fmt.Println(err)
		os.Exit(7)
	}

	if config.Users == nil {
		config.Users = make(map[string]string)
	}
	_, exists := config.Users[username]

	config.Users[username] = hashedPassword

	newFile, err := config.Export()
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
		config.Save(configPath)

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
