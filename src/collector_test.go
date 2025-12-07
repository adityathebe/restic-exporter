package main

import (
	"compress/bzip2"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func ensureRestic(t *testing.T) string {
	t.Helper()

	// First check if restic is already installed on the system
	if path, err := exec.LookPath("restic"); err == nil {
		t.Logf("Using system restic from %s", path)
		return path
	}

	// Restic not found, download it
	t.Log("Restic not found in PATH, downloading...")

	tmpDir := t.TempDir()
	resticPath := filepath.Join(tmpDir, "restic")

	// Determine architecture
	arch := runtime.GOARCH
	goos := runtime.GOOS

	// Map Go arch names to restic release names
	archMap := map[string]string{
		"amd64": "amd64",
		"arm64": "arm64",
		"386":   "386",
		"arm":   "arm",
	}

	resticArch, ok := archMap[arch]
	if !ok {
		t.Fatalf("unsupported architecture: %s", arch)
	}

	version := "0.18.1"
	url := fmt.Sprintf("https://github.com/restic/restic/releases/download/v%s/restic_%s_%s_%s.bz2",
		version, version, goos, resticArch)

	t.Logf("Downloading restic from %s", url)

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("failed to download restic: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to download restic: HTTP %d", resp.StatusCode)
	}

	// Decompress bz2
	bzReader := bzip2.NewReader(resp.Body)

	outFile, err := os.OpenFile(resticPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		t.Fatalf("failed to create restic binary: %v", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, bzReader); err != nil {
		t.Fatalf("failed to extract restic: %v", err)
	}

	t.Logf("Downloaded restic to %s", resticPath)
	return resticPath
}

func TestCollectorIncludesOnlySelectedClients(t *testing.T) {
	resticPath := ensureRestic(t)

	logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	repoDir := t.TempDir()
	sourceDir := t.TempDir()
	password := "testpass"

	runResticCmd(t, resticPath, repoDir, password, "init")

	file1 := filepath.Join(sourceDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("first backup"), 0o600); err != nil {
		t.Fatalf("write file1: %v", err)
	}
	runResticCmd(t, resticPath, repoDir, password, "--host", "client-1", "backup", "--tag", "keep", sourceDir)

	// Second client should be included.
	if err := os.WriteFile(file1, []byte("second backup"), 0o600); err != nil {
		t.Fatalf("rewrite file1: %v", err)
	}
	time.Sleep(1 * time.Second)
	runResticCmd(t, resticPath, repoDir, password, "--host", "client-2", "backup", "--tag", "keep", sourceDir)

	// Third client should be excluded by filter.
	if err := os.WriteFile(file1, []byte("third backup"), 0o600); err != nil {
		t.Fatalf("rewrite file1 (third): %v", err)
	}
	time.Sleep(1 * time.Second)
	runResticCmd(t, resticPath, repoDir, password, "--host", "client-3", "backup", "--tag", "keep", sourceDir)

	cfg := config{
		Repository:       repoDir,
		Password:         password,
		ResticBinaryPath: resticPath,
		DisableCheck:     true,
		DisableLocks:     true,
		IncludeClients:   []string{"client-1", "client-2"},
		IncludePaths:     false,
		InsecureTLS:      false,
		RefreshInterval:  time.Second,
	}

	collector := newResticCollector(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	m, err := collector.collectMetrics(ctx)
	if err != nil {
		t.Fatalf("collect metrics: %v", err)
	}

	if len(m.Clients) != 2 {
		t.Fatalf("expected 2 client snapshots after filtering, got %d", len(m.Clients))
	}

	hostSet := map[string]struct{}{
		"client-1": {},
		"client-2": {},
	}
	for _, client := range m.Clients {
		if _, ok := hostSet[client.Hostname]; !ok {
			t.Fatalf("unexpected client hostname %q in filtered results", client.Hostname)
		}
	}
	if m.SnapshotsTotal != 2 {
		t.Fatalf("expected total snapshots 2 after filtering, got %v", m.SnapshotsTotal)
	}
}

func TestCollectorErrorMetric(t *testing.T) {
	logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Use an invalid repository path that will cause an error
	cfg := config{
		Repository:       "/nonexistent/repo/path",
		Password:         "testpass",
		ResticBinaryPath: "restic",
		DisableCheck:     true,
		DisableLocks:     true,
		IncludePaths:     false,
		InsecureTLS:      false,
		RefreshInterval:  time.Second,
	}

	collector := newResticCollector(cfg)

	// Initially, should not be ready
	if collector.Ready() {
		t.Fatal("collector should not be ready initially")
	}

	// Trigger refresh with invalid repo - should not exit
	collector.Refresh()

	// After refresh, should be ready (even with error)
	if !collector.Ready() {
		t.Fatal("collector should be ready after refresh, even on error")
	}

	// Check that error metric is set to 1
	collector.mu.RLock()
	errorMetric := collector.metrics.ScrapeError
	collector.mu.RUnlock()

	if errorMetric != 1 {
		t.Fatalf("expected error metric to be 1, got %v", errorMetric)
	}
}

func TestCollectorErrorMetricClearsOnSuccess(t *testing.T) {
	resticPath := ensureRestic(t)
	logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	repoDir := t.TempDir()
	sourceDir := t.TempDir()
	password := "testpass"

	runResticCmd(t, resticPath, repoDir, password, "init")

	file1 := filepath.Join(sourceDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("test backup"), 0o600); err != nil {
		t.Fatalf("write file1: %v", err)
	}
	runResticCmd(t, resticPath, repoDir, password, "backup", sourceDir)

	cfg := config{
		Repository:       repoDir,
		Password:         password,
		ResticBinaryPath: resticPath,
		DisableCheck:     true,
		DisableLocks:     true,
		IncludePaths:     false,
		InsecureTLS:      false,
		RefreshInterval:  time.Second,
	}

	collector := newResticCollector(cfg)

	// Set an initial error state by manually setting the error metric
	collector.mu.Lock()
	collector.metrics.ScrapeError = 1
	collector.mu.Unlock()
	collector.ready.Store(true)

	// Now do a successful refresh
	collector.Refresh()

	// Check that error metric is cleared (set to 0) on success
	collector.mu.RLock()
	errorMetric := collector.metrics.ScrapeError
	collector.mu.RUnlock()

	if errorMetric != 0 {
		t.Fatalf("expected error metric to be 0 on successful scrape, got %v", errorMetric)
	}
}

func runResticCmd(t *testing.T, resticPath, repoDir, password string, args ...string) {
	t.Helper()

	fullArgs := []string{"-r", repoDir, "--no-lock"}
	fullArgs = append(fullArgs, args...)

	cmd := exec.Command(resticPath, fullArgs...)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("restic %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
