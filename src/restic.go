package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

type resticStats struct {
	TotalSize      float64 `json:"total_size"`
	TotalFileCount float64 `json:"total_file_count"`
}

type resticStatsRawData struct {
	TotalSize              float64 `json:"total_size"`
	TotalUncompressedSize  float64 `json:"total_uncompressed_size"`
	CompressionRatio       float64 `json:"compression_ratio"`
	CompressionProgress    float64 `json:"compression_progress"`
	CompressionSpaceSaving float64 `json:"compression_space_saving"`
	TotalBlobCount         float64 `json:"total_blob_count"`
	SnapshotsCount         float64 `json:"snapshots_count"`
}

type snapshot struct {
	ID             string   `json:"id"`
	Time           string   `json:"time"`
	Paths          []string `json:"paths"`
	Tags           []string `json:"tags"`
	Hostname       string   `json:"hostname"`
	Username       string   `json:"username"`
	ProgramVersion string   `json:"program_version"`
	Hash           string   `json:"-"`
	Timestamp      float64  `json:"-"`
}

type resticClient struct {
	repository   string
	password     string
	passwordFile string
	insecureTLS  bool
	binaryPath   string
	statsCache   map[string]resticStats
	statsMu      sync.Mutex
}

func newResticClient(repository, password, passwordFile string, insecureTLS bool) *resticClient {
	return &resticClient{
		repository:   repository,
		password:     password,
		passwordFile: passwordFile,
		insecureTLS:  insecureTLS,
		binaryPath:   "restic", // default to "restic" in PATH
		statsCache:   make(map[string]resticStats),
	}
}

func (r *resticClient) resticBaseArgs() []string {
	args := []string{"-r", r.repository, "--no-lock"}
	if r.passwordFile != "" {
		args = append(args, "-p", r.passwordFile)
	}
	return args
}

func (r *resticClient) getSnapshots(ctx context.Context) ([]snapshot, error) {
	args := append(r.resticBaseArgs(), "snapshots", "--json")
	if r.insecureTLS {
		args = append(args, "--insecure-tls")
	}
	logger.Debug("Running restic snapshots")

	stdout, stderr, err := r.runRestic(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("Error executing restic snapshot command: %s", formatCommandError(err, stderr))
	}

	var snaps []snapshot
	if err := json.Unmarshal(stdout, &snaps); err != nil {
		return nil, fmt.Errorf("decode restic snapshots: %w", err)
	}

	for i := range snaps {
		snaps[i].Hash = calcSnapshotHash(snaps[i])
	}

	return snaps, nil
}

func (r *resticClient) getRestoreSize(ctx context.Context, snapshotID string) (resticStats, error) {
	if snapshotID != "" {
		r.statsMu.Lock()
		stats, ok := r.statsCache[snapshotID]
		r.statsMu.Unlock()
		if ok {
			return stats, nil
		}
	}

	args := append(r.resticBaseArgs(), "stats", "--json")
	if snapshotID != "" {
		args = append(args, snapshotID)
	}
	if r.insecureTLS {
		args = append(args, "--insecure-tls")
	}
	logger.Debug("Running restic stats", "snapshot_id", snapshotID)

	stdout, stderr, err := r.runRestic(ctx, args)
	if err != nil {
		return resticStats{}, fmt.Errorf("Error executing restic stats command: %s", formatCommandError(err, stderr))
	}

	var stats resticStats
	if err := json.Unmarshal(stdout, &stats); err != nil {
		return resticStats{}, fmt.Errorf("decode restic stats: %w", err)
	}

	if snapshotID != "" {
		r.statsMu.Lock()
		r.statsCache[snapshotID] = stats
		r.statsMu.Unlock()
	}

	return stats, nil
}

func (r *resticClient) getStatsRawData(ctx context.Context) (resticStatsRawData, error) {
	args := append(r.resticBaseArgs(), "stats", "--mode", "raw-data", "--json")
	if r.insecureTLS {
		args = append(args, "--insecure-tls")
	}
	logger.Debug("Running restic stats --mode raw-data")

	stdout, stderr, err := r.runRestic(ctx, args)
	if err != nil {
		return resticStatsRawData{}, fmt.Errorf("Error executing restic stats raw-data command: %s", formatCommandError(err, stderr))
	}

	var stats resticStatsRawData
	if err := json.Unmarshal(stdout, &stats); err != nil {
		return resticStatsRawData{}, fmt.Errorf("decode restic stats raw-data: %w", err)
	}

	return stats, nil
}

func (r *resticClient) evictStaleStatsCache(validSnapshotIDs map[string]bool) {
	r.statsMu.Lock()
	defer r.statsMu.Unlock()

	var evicted int
	for snapshotID := range r.statsCache {
		if !validSnapshotIDs[snapshotID] {
			delete(r.statsCache, snapshotID)
			evicted++
		}
	}

	if evicted > 0 {
		logger.Debug("Evicted stale stats cache entries", "count", evicted, "remaining", len(r.statsCache))
	}
}

func (r *resticClient) getCheck(ctx context.Context) (float64, error) {
	args := append(r.resticBaseArgs(), "check")
	if r.insecureTLS {
		args = append(args, "--insecure-tls")
	}
	logger.Debug("Running restic check")

	_, stderr, err := r.runRestic(ctx, args)
	if err != nil {
		logger.Warn("Error checking repository health", "error", formatCommandError(err, stderr))
		return 0, nil
	}

	return 1, nil
}

func (r *resticClient) runRestic(ctx context.Context, args []string) ([]byte, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, r.binaryPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if r.password != "" && r.passwordFile == "" {
		cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+r.password)
	}

	err := cmd.Run()
	return stdout.Bytes(), stderr.String(), err
}
