package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/blamoo/sugoi/internal/catalog"
	"github.com/blamoo/sugoi/internal/config"
	"github.com/blamoo/sugoi/internal/library"
	"github.com/blamoo/sugoi/internal/search"
	"github.com/blamoo/sugoi/internal/web"
)

//go:generate go run cmd/static/main.go

const shutdownRequestTimeout = 10 * time.Second
const shutdownReindexTimeout = 30 * time.Second

func main() {
	configPath := flag.String("c", "./config/sugoi.json", "Path to the configuration file. Default: ./config/sugoi.json")
	user := flag.Bool("u", false, "Adds a new user or changes de password of an existing user on your config file.")
	flag.Parse()

	InitializeLogging()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		slog.Error("loading config", "path", *configPath, "err", err)
		os.Exit(1)
	}
	SetDebugLogging(cfg.Debug)

	if *user {
		ManageUsers(cfg, *configPath)
		return
	}

	lib, err := library.Load(cfg)
	if err != nil {
		slog.Error("loading the file list", "err", err)
		os.Exit(5)
	}
	slog.Info("file list loaded", "files", len(lib.List))

	index, err := search.Open(cfg.DatabaseDir)
	if err != nil {
		slog.Error("opening the search index", "err", err)
		os.Exit(8)
	}

	cat := catalog.New(cfg, lib, index)

	err = cat.Autocomplete.Rebuild()
	if err != nil {
		slog.Error("building autocomplete", "err", err)
	}

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort),
		Handler: web.NewServer(cfg, cat, ".").Handler(),
	}

	httproot := fmt.Sprintf("http://%s:%d/", cfg.ServerHost, cfg.ServerPort)
	if cfg.ServerPort == 80 {
		httproot = fmt.Sprintf("http://%s/", cfg.ServerHost)
	}
	slog.Info("uwu, listening", "url", httproot)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err = <-serveErr:
		// Failed to start (port in use, permission denied...).
		slog.Error("starting the server", "addr", server.Addr, "err", err)
		os.Exit(4)

	case <-ctx.Done():
		stop() // a second Ctrl+C kills the process immediately
		Shutdown(server, cat)
	}

	slog.Info("owo, stopped")
}

// loadConfig reads the config file. When the session key is missing it prints
// a freshly generated one to paste into the file.
func loadConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)

	var keyErr config.SessionKeyError
	if errors.As(err, &keyErr) {
		fmt.Println(keyErr.Error())
		fmt.Println("Like this:")
		fmt.Println(keyErr.Suggested)
		return nil, fmt.Errorf("update your configuration file with a valid value, then run sugoi again")
	}

	return cfg, err
}

// Shutdown stops accepting connections, waits for in-flight requests, stops a
// running reindex and closes the search index so it's left consistent.
func Shutdown(server *http.Server, cat *catalog.Catalog) {
	slog.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownRequestTimeout)
	defer cancel()
	err := server.Shutdown(ctx)
	if err != nil {
		slog.Warn("some requests didn't finish in time", "err", err)
	}

	if cat.Reindex.Running() {
		slog.Warn("cancelling the running reindex; the index will be incomplete until a new reindex")
		cat.Reindex.Cancel()
		if !cat.Reindex.Wait(shutdownReindexTimeout) {
			slog.Warn("reindex didn't stop in time")
		}
	}

	err = cat.Index.Close()
	if err != nil {
		slog.Error("closing the search index", "err", err)
	}
}
