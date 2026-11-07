package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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

	err = RebuildAutocomplete()
	if err != nil {
		fmt.Println("autocomplete:", err)
	}

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
		httproot = fmt.Sprintf("http://%s:%d/", config.ServerHost, config.ServerPort)
	}

	fmt.Printf("Listening on %s\n", httproot)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", config.ServerHost, config.ServerPort),
		Handler: router,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err = <-serveErr:
		// Failed to start (port in use, permission denied...).
		fmt.Println(err)
		os.Exit(4)

	case <-ctx.Done():
		stop() // a second Ctrl+C kills the process immediately
		Shutdown(server)
	}

	fmt.Println("owo")
}

const shutdownRequestTimeout = 10 * time.Second
const shutdownReindexTimeout = 30 * time.Second

// Shutdown stops accepting connections, waits for in-flight requests, stops a
// running reindex and closes the search index so it's left consistent.
func Shutdown(server *http.Server) {
	fmt.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownRequestTimeout)
	defer cancel()
	err := server.Shutdown(ctx)
	if err != nil {
		fmt.Println("Some requests didn't finish in time:", err)
	}

	if reindexJob.Running {
		fmt.Println("Cancelling the running reindex; the index will be incomplete until a new reindex")
		reindexJob.RequestCancel = true

		deadline := time.Now().Add(shutdownReindexTimeout)
		for reindexJob.Running && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
		}
		if reindexJob.Running {
			fmt.Println("Reindex didn't stop in time")
		}
	}

	err = bleveIndex.Close()
	if err != nil {
		fmt.Println("Closing the index:", err)
	}
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
		err = config.Save(configPath)

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
