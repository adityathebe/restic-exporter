package main

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectorIncludesOnlySelectedClients(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic binary required for integration test: " + err.Error())
	}

	logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	repoDir := t.TempDir()
	sourceDir := t.TempDir()
	password := "testpass"

	runResticCmd(t, repoDir, password, "init")

	file1 := filepath.Join(sourceDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("first backup"), 0o600); err != nil {
		t.Fatalf("write file1: %v", err)
	}
	runResticCmd(t, repoDir, password, "--host", "client-1", "backup", "--tag", "keep", sourceDir)

	// Second client should be included.
	if err := os.WriteFile(file1, []byte("second backup"), 0o600); err != nil {
		t.Fatalf("rewrite file1: %v", err)
	}
	time.Sleep(1 * time.Second)
	runResticCmd(t, repoDir, password, "--host", "client-2", "backup", "--tag", "keep", sourceDir)

	// Third client should be excluded by filter.
	if err := os.WriteFile(file1, []byte("third backup"), 0o600); err != nil {
		t.Fatalf("rewrite file1 (third): %v", err)
	}
	time.Sleep(1 * time.Second)
	runResticCmd(t, repoDir, password, "--host", "client-3", "backup", "--tag", "keep", sourceDir)

	cfg := config{
		Repository:      repoDir,
		Password:        password,
		DisableCheck:    true,
		DisableStats:    true,
		DisableLocks:    true,
		IncludeClients:  []string{"client-1", "client-2"},
		IncludePaths:    false,
		InsecureTLS:     false,
		RefreshInterval: time.Second,
	}

	collector := newResticCollector(cfg)
	m, err := collector.collectMetrics()
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

func runResticCmd(t *testing.T, repoDir, password string, args ...string) {
	t.Helper()

	fullArgs := []string{"-r", repoDir, "--no-lock"}
	fullArgs = append(fullArgs, args...)

	cmd := exec.Command("restic", fullArgs...)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("restic %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
