package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type clientMetrics struct {
	Hostname       string
	Username       string
	Version        string
	SnapshotHash   string
	SnapshotTag    string
	SnapshotTags   string
	SnapshotPaths  string
	Timestamp      float64
	FirstTimestamp float64
	SizeTotal      float64
	FilesTotal     float64
	SnapshotsTotal float64
}

type metrics struct {
	CheckSuccess                    float64
	LocksTotal                      float64
	Clients                         []clientMetrics
	SnapshotsTotal                  float64
	Duration                        float64
	ScrapeTimestamp                 float64
	RepositoryTotalSize             float64
	RepositoryTotalUncompressedSize float64
	CompressionRatio                float64
	StatsDuration                   float64
}

type resticCollector struct {
	cfg     config
	restic  *resticClient
	metrics metrics
	mu      sync.RWMutex
	ready   atomic.Bool

	checkDesc                           *prometheus.Desc
	locksDesc                           *prometheus.Desc
	snapshotsDesc                       *prometheus.Desc
	backupTimestampDesc                 *prometheus.Desc
	backupFirstTimestampDesc            *prometheus.Desc
	backupFilesTotalDesc                *prometheus.Desc
	backupSizeTotalDesc                 *prometheus.Desc
	backupSnapshotsDesc                 *prometheus.Desc
	scrapeDurationDesc                  *prometheus.Desc
	scrapeTimestampDesc                 *prometheus.Desc
	repositoryTotalSizeDesc             *prometheus.Desc
	repositoryTotalUncompressedSizeDesc *prometheus.Desc
	compressionRatioDesc                *prometheus.Desc
	statsDurationDesc                   *prometheus.Desc
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

	resticClient := newResticClient(cfg.Repository, cfg.Password, cfg.PasswordFile, cfg.InsecureTLS)
	if cfg.ResticBinaryPath != "" {
		resticClient.binaryPath = cfg.ResticBinaryPath
	}

	return &resticCollector{
		cfg:     cfg,
		restic:  resticClient,
		metrics: metrics{},
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
		backupFirstTimestampDesc: prometheus.NewDesc(
			"restic_backup_first_timestamp",
			"Timestamp of the first backup",
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
		scrapeTimestampDesc: prometheus.NewDesc(
			"restic_last_scrape_timestamp_seconds",
			"Unix timestamp of the last metrics scrape from the restic repository",
			nil,
			nil,
		),
		repositoryTotalSizeDesc: prometheus.NewDesc(
			"restic_repository_total_size_bytes",
			"Total size of the repository in bytes (raw data)",
			nil,
			nil,
		),
		repositoryTotalUncompressedSizeDesc: prometheus.NewDesc(
			"restic_repository_total_uncompressed_size_bytes",
			"Total uncompressed size of the repository in bytes",
			nil,
			nil,
		),
		compressionRatioDesc: prometheus.NewDesc(
			"restic_compression_ratio",
			"Compression ratio of the repository",
			nil,
			nil,
		),
		statsDurationDesc: prometheus.NewDesc(
			"restic_stats_duration_seconds",
			"Duration to run the stats command",
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
	ch <- c.backupFirstTimestampDesc
	ch <- c.backupFilesTotalDesc
	ch <- c.backupSizeTotalDesc
	ch <- c.backupSnapshotsDesc
	ch <- c.scrapeDurationDesc
	ch <- c.scrapeTimestampDesc
	ch <- c.repositoryTotalSizeDesc
	ch <- c.repositoryTotalUncompressedSizeDesc
	ch <- c.compressionRatioDesc
	ch <- c.statsDurationDesc
}

func (c *resticCollector) Collect(ch chan<- prometheus.Metric) {
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
		ch <- prometheus.MustNewConstMetric(c.backupFirstTimestampDesc, prometheus.GaugeValue, client.FirstTimestamp, labels...)
		ch <- prometheus.MustNewConstMetric(c.backupFilesTotalDesc, prometheus.CounterValue, client.FilesTotal, labels...)
		ch <- prometheus.MustNewConstMetric(c.backupSizeTotalDesc, prometheus.CounterValue, client.SizeTotal, labels...)
		ch <- prometheus.MustNewConstMetric(c.backupSnapshotsDesc, prometheus.CounterValue, client.SnapshotsTotal, labels...)
	}

	ch <- prometheus.MustNewConstMetric(c.scrapeDurationDesc, prometheus.GaugeValue, m.Duration)
	ch <- prometheus.MustNewConstMetric(c.scrapeTimestampDesc, prometheus.GaugeValue, m.ScrapeTimestamp)

	if m.RepositoryTotalSize >= 0 {
		ch <- prometheus.MustNewConstMetric(c.repositoryTotalSizeDesc, prometheus.GaugeValue, m.RepositoryTotalSize)
	}
	if m.RepositoryTotalUncompressedSize >= 0 {
		ch <- prometheus.MustNewConstMetric(c.repositoryTotalUncompressedSizeDesc, prometheus.GaugeValue, m.RepositoryTotalUncompressedSize)
	}
	if m.CompressionRatio >= 0 {
		ch <- prometheus.MustNewConstMetric(c.compressionRatioDesc, prometheus.GaugeValue, m.CompressionRatio)
	}
	if m.StatsDuration >= 0 {
		ch <- prometheus.MustNewConstMetric(c.statsDurationDesc, prometheus.GaugeValue, m.StatsDuration)
	}
}

func (c *resticCollector) Refresh() {
	logger.Debug("Starting metrics refresh")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	m, err := c.collectMetrics(ctx)
	if err != nil {
		logger.Error("Unable to collect metrics from Restic", "error", err)
		os.Exit(1)
	}

	c.mu.Lock()
	c.metrics = m
	c.mu.Unlock()
	c.ready.Store(true)
	logger.Debug("Metrics refresh completed")
}

func (c *resticCollector) Ready() bool {
	return c.ready.Load()
}

func (c *resticCollector) collectMetrics(ctx context.Context) (metrics, error) {
	start := time.Now()

	allSnapshots, err := c.restic.getSnapshots(ctx)
	if err != nil {
		return metrics{}, err
	}
	allSnapshots = c.filterSnapshotsByClient(allSnapshots)
	logger.Debug("Loaded total snapshots", "count", len(allSnapshots))

	// Build set of valid snapshot IDs for cache eviction
	validSnapshotIDs := make(map[string]bool, len(allSnapshots))
	for _, snap := range allSnapshots {
		validSnapshotIDs[snap.ID] = true
	}

	// Evict cache entries for snapshots that no longer exist
	c.restic.evictStaleStatsCache(validSnapshotIDs)

	snapshotCounts := make(map[string]int)
	for _, snap := range allSnapshots {
		snapshotCounts[snap.Hash]++
	}

	latestSnapshots := make(map[string]snapshot)
	firstSnapshots := make(map[string]snapshot)
	for _, snap := range allSnapshots {
		ts, err := parseResticTime(snap.Time)
		if err != nil {
			return metrics{}, fmt.Errorf("parse snapshot time %q: %w", snap.Time, err)
		}
		snap.Timestamp = ts
		if existing, ok := latestSnapshots[snap.Hash]; !ok || snap.Timestamp > existing.Timestamp {
			latestSnapshots[snap.Hash] = snap
		}
		if existing, ok := firstSnapshots[snap.Hash]; !ok || snap.Timestamp < existing.Timestamp {
			firstSnapshots[snap.Hash] = snap
		}
	}
	logger.Debug("selected latest snapshot entries", "count", len(latestSnapshots))

	var clients []clientMetrics
	for _, snap := range latestSnapshots {
		stats := resticStats{TotalSize: -1, TotalFileCount: -1}
		if !c.cfg.DisableStatsSnapshotRestoreSize {
			stats, err = c.restic.getRestoreSize(ctx, snap.ID)
			if err != nil {
				return metrics{}, err
			}
		}

		firstSnap := firstSnapshots[snap.Hash]
		clients = append(clients, clientMetrics{
			Hostname:       snap.Hostname,
			Username:       snap.Username,
			Version:        snap.ProgramVersion,
			SnapshotHash:   snap.Hash,
			SnapshotTag:    firstTag(snap.Tags),
			SnapshotTags:   strings.Join(snap.Tags, ","),
			SnapshotPaths:  snapshotPaths(c.cfg.IncludePaths, snap.Paths),
			Timestamp:      snap.Timestamp,
			FirstTimestamp: firstSnap.Timestamp,
			SizeTotal:      stats.TotalSize,
			FilesTotal:     stats.TotalFileCount,
			SnapshotsTotal: float64(snapshotCounts[snap.Hash]),
		})
	}

	var checkSuccess float64
	if c.cfg.DisableCheck {
		checkSuccess = 2
	} else {
		checkSuccess, err = c.restic.getCheck(ctx)
		if err != nil {
			return metrics{}, err
		}
	}
	logger.Debug("Check success metric collected", "value", checkSuccess)

	var locksTotal float64
	if c.cfg.DisableLocks {
		locksTotal = 0
	} else {
		locksTotal, err = c.restic.getLocks(ctx)
		if err != nil {
			return metrics{}, err
		}
	}
	logger.Debug("Locks collected", "value", locksTotal)

	var statsRawData resticStatsRawData
	var statsDuration float64
	if c.cfg.DisableStatsRawData {
		statsRawData = resticStatsRawData{TotalSize: -1, TotalUncompressedSize: -1, CompressionRatio: -1}
		statsDuration = -1
	} else {
		statsStart := time.Now()
		statsRawData, err = c.restic.getStatsRawData(ctx)
		if err != nil {
			return metrics{}, err
		}
		statsDuration = time.Since(statsStart).Seconds()
	}
	logger.Debug("Stats raw data collected", "total_size", statsRawData.TotalSize, "compression_ratio", statsRawData.CompressionRatio)

	return metrics{
		CheckSuccess:                    checkSuccess,
		LocksTotal:                      locksTotal,
		Clients:                         clients,
		SnapshotsTotal:                  float64(len(allSnapshots)),
		Duration:                        time.Since(start).Seconds(),
		ScrapeTimestamp:                 float64(time.Now().Unix()),
		RepositoryTotalSize:             statsRawData.TotalSize,
		RepositoryTotalUncompressedSize: statsRawData.TotalUncompressedSize,
		CompressionRatio:                statsRawData.CompressionRatio,
		StatsDuration:                   statsDuration,
	}, nil
}

func (c *resticCollector) filterSnapshotsByClient(snaps []snapshot) []snapshot {
	if len(c.cfg.IncludeClients) == 0 {
		return snaps
	}

	included := make(map[string]struct{}, len(c.cfg.IncludeClients))
	for _, client := range c.cfg.IncludeClients {
		if client == "" {
			continue
		}
		included[client] = struct{}{}
	}

	filtered := make([]snapshot, 0, len(snaps))
	for _, snap := range snaps {
		if _, ok := included[snap.Hostname]; ok {
			filtered = append(filtered, snap)
		}
	}

	if len(filtered) != len(snaps) {
		logger.Debug("Filtered snapshots by client", "include_clients", c.cfg.IncludeClients, "before", len(snaps), "after", len(filtered))
	}

	return filtered
}
