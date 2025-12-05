package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type resticStats struct {
	TotalSize      float64 `json:"total_size"`
	TotalFileCount float64 `json:"total_file_count"`
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

type clientMetrics struct {
	Hostname       string
	Username       string
	Version        string
	SnapshotHash   string
	SnapshotTag    string
	SnapshotTags   string
	SnapshotPaths  string
	Timestamp      float64
	SizeTotal      float64
	FilesTotal     float64
	SnapshotsTotal float64
}

type metrics struct {
	CheckSuccess   float64
	LocksTotal     float64
	Clients        []clientMetrics
	SnapshotsTotal float64
	Duration       float64
}

type resticCollector struct {
	cfg        config
	statsCache map[string]resticStats
	metrics    metrics
	mu         sync.RWMutex
	statsMu    sync.Mutex

	checkDesc            *prometheus.Desc
	locksDesc            *prometheus.Desc
	snapshotsDesc        *prometheus.Desc
	backupTimestampDesc  *prometheus.Desc
	backupFilesTotalDesc *prometheus.Desc
	backupSizeTotalDesc  *prometheus.Desc
	backupSnapshotsDesc  *prometheus.Desc
	scrapeDurationDesc   *prometheus.Desc
}

func newResticCollector(cfg config) *resticCollector {
	commonLabels := []string{
		"client_hostname",
		"client_username",
		"client_version",
		"snapshot_hash",
		"snapshot_tag",
		"snapshot_tags",
		"snapshot_paths",
	}

	return &resticCollector{
		cfg:        cfg,
		statsCache: make(map[string]resticStats),
		metrics:    metrics{},
		checkDesc: prometheus.NewDesc(
			"restic_check_success",
			"Result of restic check operation in the repository",
			nil,
			nil,
		),
		locksDesc: prometheus.NewDesc(
			"restic_locks_total",
			"Total number of locks in the repository",
			nil,
			nil,
		),
		snapshotsDesc: prometheus.NewDesc(
			"restic_snapshots_total",
			"Total number of snapshots in the repository",
			nil,
			nil,
		),
		backupTimestampDesc: prometheus.NewDesc(
			"restic_backup_timestamp",
			"Timestamp of the last backup",
			commonLabels,
			nil,
		),
		backupFilesTotalDesc: prometheus.NewDesc(
			"restic_backup_files_total",
			"Number of files in the backup",
			commonLabels,
			nil,
		),
		backupSizeTotalDesc: prometheus.NewDesc(
			"restic_backup_size_total",
			"Total size of backup in bytes",
			commonLabels,
			nil,
		),
		backupSnapshotsDesc: prometheus.NewDesc(
			"restic_backup_snapshots_total",
			"Total number of snapshots",
			commonLabels,
			nil,
		),
		scrapeDurationDesc: prometheus.NewDesc(
			"restic_scrape_duration_seconds",
			"Amount of time each scrape takes",
			nil,
			nil,
		),
	}
}

func (c *resticCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.checkDesc
	ch <- c.locksDesc
	ch <- c.snapshotsDesc
	ch <- c.backupTimestampDesc
	ch <- c.backupFilesTotalDesc
	ch <- c.backupSizeTotalDesc
	ch <- c.backupSnapshotsDesc
	ch <- c.scrapeDurationDesc
}

func (c *resticCollector) Collect(ch chan<- prometheus.Metric) {
	logger.Debug("Incoming scrape request")
	c.mu.RLock()
	m := c.metrics
	c.mu.RUnlock()

	ch <- prometheus.MustNewConstMetric(c.checkDesc, prometheus.GaugeValue, m.CheckSuccess)
	ch <- prometheus.MustNewConstMetric(c.locksDesc, prometheus.CounterValue, m.LocksTotal)
	ch <- prometheus.MustNewConstMetric(c.snapshotsDesc, prometheus.CounterValue, m.SnapshotsTotal)

	for _, client := range m.Clients {
		labels := []string{
			client.Hostname,
			client.Username,
			client.Version,
			client.SnapshotHash,
			client.SnapshotTag,
			client.SnapshotTags,
			client.SnapshotPaths,
		}
		ch <- prometheus.MustNewConstMetric(c.backupTimestampDesc, prometheus.GaugeValue, client.Timestamp, labels...)
		ch <- prometheus.MustNewConstMetric(c.backupFilesTotalDesc, prometheus.CounterValue, client.FilesTotal, labels...)
		ch <- prometheus.MustNewConstMetric(c.backupSizeTotalDesc, prometheus.CounterValue, client.SizeTotal, labels...)
		ch <- prometheus.MustNewConstMetric(c.backupSnapshotsDesc, prometheus.CounterValue, client.SnapshotsTotal, labels...)
	}

	ch <- prometheus.MustNewConstMetric(c.scrapeDurationDesc, prometheus.GaugeValue, m.Duration)
}

func (c *resticCollector) Refresh(exitOnError bool) {
	logger.Debug("Starting metrics refresh")
	m, err := c.collectMetrics()
	if err != nil {
		logger.Error("Unable to collect metrics from Restic", "error", err)
		os.Exit(1)
	}

	c.mu.Lock()
	c.metrics = m
	c.mu.Unlock()
	logger.Debug("Metrics refresh completed")
}

func (c *resticCollector) collectMetrics() (metrics, error) {
	start := time.Now()

	allSnapshots, err := c.getSnapshots(false)
	if err != nil {
		return metrics{}, err
	}
	logger.Debug("Loaded total snapshots", "count", len(allSnapshots))

	snapshotCounts := make(map[string]int)
	for _, snap := range allSnapshots {
		snapshotCounts[snap.Hash]++
	}

	latestSnapshotsRaw, err := c.getSnapshots(true)
	if err != nil {
		return metrics{}, err
	}

	latestSnapshots := make(map[string]snapshot)
	for _, snap := range latestSnapshotsRaw {
		ts, err := parseResticTime(snap.Time)
		if err != nil {
			return metrics{}, fmt.Errorf("parse snapshot time %q: %w", snap.Time, err)
		}
		snap.Timestamp = ts
		if existing, ok := latestSnapshots[snap.Hash]; !ok || snap.Timestamp > existing.Timestamp {
			latestSnapshots[snap.Hash] = snap
		}
	}
	logger.Debug("Selected latest snapshot entries", "count", len(latestSnapshots))

	var clients []clientMetrics
	for _, snap := range latestSnapshots {
		var stats resticStats
		if c.cfg.DisableStats {
			stats = resticStats{TotalSize: -1, TotalFileCount: -1}
		} else {
			stats, err = c.getStats(snap.ID)
			if err != nil {
				return metrics{}, err
			}
		}

		clients = append(clients, clientMetrics{
			Hostname:       snap.Hostname,
			Username:       snap.Username,
			Version:        snap.ProgramVersion,
			SnapshotHash:   snap.Hash,
			SnapshotTag:    firstTag(snap.Tags),
			SnapshotTags:   strings.Join(snap.Tags, ","),
			SnapshotPaths:  snapshotPaths(c.cfg.IncludePaths, snap.Paths),
			Timestamp:      snap.Timestamp,
			SizeTotal:      stats.TotalSize,
			FilesTotal:     stats.TotalFileCount,
			SnapshotsTotal: float64(snapshotCounts[snap.Hash]),
		})
	}

	var checkSuccess float64
	if c.cfg.DisableCheck {
		checkSuccess = 2
	} else {
		checkSuccess, err = c.getCheck()
		if err != nil {
			return metrics{}, err
		}
	}
	logger.Debug("Check success metric collected", "value", checkSuccess)

	var locksTotal float64
	if c.cfg.DisableLocks {
		locksTotal = 0
	} else {
		locksTotal, err = c.getLocks()
		if err != nil {
			return metrics{}, err
		}
	}
	logger.Debug("Locks collected", "value", locksTotal)

	return metrics{
		CheckSuccess:   checkSuccess,
		LocksTotal:     locksTotal,
		Clients:        clients,
		SnapshotsTotal: float64(len(allSnapshots)),
		Duration:       time.Since(start).Seconds(),
	}, nil
}

func (c *resticCollector) resticBaseArgs() []string {
	args := []string{"-r", c.cfg.Repository, "--no-lock"}
	if c.cfg.PasswordFile != "" {
		args = append(args, "-p", c.cfg.PasswordFile)
	}
	return args
}

func (c *resticCollector) getSnapshots(onlyLatest bool) ([]snapshot, error) {
	args := append(c.resticBaseArgs(), "snapshots", "--json")
	if onlyLatest {
		args = append(args, "--latest", "1")
	}
	if c.cfg.InsecureTLS {
		args = append(args, "--insecure-tls")
	}
	logger.Debug("Running restic snapshots", "latest_only", onlyLatest)

	stdout, stderr, err := c.runRestic(args)
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

func (c *resticCollector) getStats(snapshotID string) (resticStats, error) {
	if snapshotID != "" {
		c.statsMu.Lock()
		stats, ok := c.statsCache[snapshotID]
		c.statsMu.Unlock()
		if ok {
			return stats, nil
		}
	}

	args := append(c.resticBaseArgs(), "stats", "--json")
	if snapshotID != "" {
		args = append(args, snapshotID)
	}
	if c.cfg.InsecureTLS {
		args = append(args, "--insecure-tls")
	}
	logger.Debug("Running restic stats", "snapshot_id", snapshotID)

	stdout, stderr, err := c.runRestic(args)
	if err != nil {
		return resticStats{}, fmt.Errorf("Error executing restic stats command: %s", formatCommandError(err, stderr))
	}

	var stats resticStats
	if err := json.Unmarshal(stdout, &stats); err != nil {
		return resticStats{}, fmt.Errorf("decode restic stats: %w", err)
	}

	if snapshotID != "" {
		c.statsMu.Lock()
		c.statsCache[snapshotID] = stats
		c.statsMu.Unlock()
	}

	return stats, nil
}

func (c *resticCollector) getCheck() (float64, error) {
	args := append(c.resticBaseArgs(), "check")
	if c.cfg.InsecureTLS {
		args = append(args, "--insecure-tls")
	}
	logger.Debug("Running restic check")

	_, stderr, err := c.runRestic(args)
	if err != nil {
		logger.Warn("Error checking repository health", "error", formatCommandError(err, stderr))
		return 0, nil
	}

	return 1, nil
}

func (c *resticCollector) getLocks() (float64, error) {
	args := append(c.resticBaseArgs(), "list", "locks")
	if c.cfg.InsecureTLS {
		args = append(args, "--insecure-tls")
	}
	logger.Debug("Running restic list locks")

	stdout, stderr, err := c.runRestic(args)
	if err != nil {
		return 0, fmt.Errorf("Error executing restic list locks command: %s", formatCommandError(err, stderr))
	}

	reLock := regexp.MustCompile(`^[a-z0-9]+$`)
	count := 0
	for _, line := range strings.Split(string(stdout), "\n") {
		if reLock.MatchString(strings.TrimSpace(line)) {
			count++
		}
	}

	return float64(count), nil
}

func (c *resticCollector) runRestic(args []string) ([]byte, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("restic", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if c.cfg.Password != "" && c.cfg.PasswordFile == "" {
		cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+c.cfg.Password)
	}

	err := cmd.Run()
	return stdout.Bytes(), stderr.String(), err
}
