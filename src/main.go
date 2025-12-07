package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var logger *slog.Logger

func newLogger(raw string) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO", "":
		level = slog.LevelInfo
	case "WARNING", "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
		fmt.Fprintf(os.Stderr, "WARN Unknown LOG_LEVEL %q, defaulting to INFO\n", raw)
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

type config struct {
	Repository                      string
	Password                        string
	PasswordFile                    string
	ResticBinaryPath                string
	ListenAddress                   string
	ListenPort                      int
	RefreshInterval                 time.Duration
	DisableCheck                    bool
	DisableLocks                    bool
	DisableStatsSnapshotRestoreSize bool
	DisableStatsRawData             bool
	IncludePaths                    bool
	InsecureTLS                     bool
	IncludeClients                  []string
}

func main() {
	logger = newLogger(os.Getenv("LOG_LEVEL"))

	repoURL := os.Getenv("RESTIC_REPOSITORY")
	if repoURL == "" {
		logger.Error("The environment variable RESTIC_REPOSITORY is mandatory")
		os.Exit(1)
	}

	password := os.Getenv("RESTIC_PASSWORD")
	passwordFile := os.Getenv("RESTIC_PASSWORD_FILE")
	if password == "" && passwordFile == "" {
		logger.Error("Define RESTIC_PASSWORD or RESTIC_PASSWORD_FILE")
		os.Exit(1)
	}

	logger.Info("Starting Restic Prometheus Exporter")
	logger.Info("It could take a while if the repository is remote")

	listenAddress := os.Getenv("LISTEN_ADDRESS")
	if listenAddress == "" {
		listenAddress = "0.0.0.0"
	}
	listenPort := envInt("LISTEN_PORT", 8001)
	if listenPort < 1 || listenPort > 65535 {
		logger.Error("LISTEN_PORT must be between 1 and 65535", "port", listenPort)
		os.Exit(1)
	}

	refreshSeconds := envInt("REFRESH_INTERVAL", 60*10) // 10 minutes
	if refreshSeconds <= 0 {
		logger.Error("REFRESH_INTERVAL must be greater than 0", "seconds", refreshSeconds)
		os.Exit(1)
	}

	disableSnapshotStats := envBool("NO_STATS_SNAPSHOT_RESTORE_SIZE")

	cfg := config{
		Repository:                      repoURL,
		Password:                        password,
		PasswordFile:                    passwordFile,
		ListenAddress:                   listenAddress,
		ListenPort:                      listenPort,
		RefreshInterval:                 time.Duration(refreshSeconds) * time.Second,
		DisableCheck:                    envBool("NO_CHECK"),
		DisableLocks:                    envBool("NO_LOCKS"),
		DisableStatsSnapshotRestoreSize: disableSnapshotStats,
		DisableStatsRawData:             envBool("NO_STATS_RAW_DATA"),
		IncludePaths:                    envBool("INCLUDE_PATHS"),
		InsecureTLS:                     envBool("INSECURE_TLS"),
		IncludeClients:                  envCSV("INCLUDE_CLIENTS"),
	}

	collector := newResticCollector(cfg)
	prometheus.MustRegister(collector)

	metricsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !collector.Ready() {
			http.Error(w, "metrics not ready", http.StatusServiceUnavailable)
			return
		}
		promhttp.Handler().ServeHTTP(w, r)
	})

	http.Handle("/metrics", metricsHandler)
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if !collector.Ready() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	addr := fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort)

	// Configure HTTP server with timeouts to prevent Slowloris attacks
	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("Serving metrics", "addr", addr)
		if err := server.ListenAndServe(); err != nil {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	go func() {
		// initial refresh without blocking the HTTP server
		collector.Refresh()

		ticker := time.NewTicker(cfg.RefreshInterval)
		defer ticker.Stop()
		for {
			logger.Info("Refreshing stats", "interval_seconds", refreshSeconds)
			<-ticker.C
			collector.Refresh()
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	logger.Info("Received shutdown signal, exiting gracefully", "signal", sig)
}
