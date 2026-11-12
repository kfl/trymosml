package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Configuration struct {
	Addr                  string
	Rootdir               string
	Command               string
	CommandArgs           []string
	SessionTimeoutSeconds int
	IdleTimeoutSeconds    int
	MaxSessions           int
	MaxMessageBytes       int64
}

func readConfig(filename string) (Configuration, error) {
	config := Configuration{
		Addr: "127.0.0.1:8080", Rootdir: "./web",
		SessionTimeoutSeconds: 1800, IdleTimeoutSeconds: 600,
		MaxSessions: 4, MaxMessageBytes: 1 << 20,
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, err
	}
	if config.Command == "" || config.Addr == "" || config.Rootdir == "" ||
		config.SessionTimeoutSeconds <= 0 || config.IdleTimeoutSeconds <= 0 ||
		config.MaxSessions <= 0 || config.MaxMessageBytes <= 0 {
		return config, fmt.Errorf("command, address, root directory and positive session limits are required")
	}
	return config, nil
}

func newHandler(ctx context.Context, config Configuration, reg *Registry) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(config.Rootdir)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/webshell", func(w http.ResponseWriter, r *http.Request) {
		serveClient(ctx, reg, config, w, r)
	})
	return mux
}

func main() {
	conf := flag.String("conf", "conf.json", "JSON configuration file")
	flag.Parse()
	config, err := readConfig(*conf)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	reg := newRegistry(config.MaxSessions)
	server := &http.Server{
		Addr: config.Addr, Handler: newHandler(ctx, config, reg),
		ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()
	log.Printf("Serving HTTP at %s (HTTPS is handled by Caddy)", config.Addr)
	select {
	case <-ctx.Done():
	case err = <-serverErr:
		cancel()
	}
	reg.stop()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
		log.Printf("HTTP shutdown: %v", shutdownErr)
	}
	// HTTP Shutdown does not wait for hijacked WebSocket connections.
	reg.wait()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
