# adityathebe/restic-exporter

[![Latest release](https://img.shields.io/github/v/release/adityathebe/restic-exporter)](https://github.com/adityathebe/restic-exporter/releases)
[![Docker Pulls](https://img.shields.io/badge/GHCR-adityathebe/restic--exporter-blue)](https://ghcr.io/adityathebe/restic-exporter)

Prometheus exporter for the [Restic](https://github.com/restic/restic) backup system.

> Thanks to [ngosang/restic-exporter](https://github.com/ngosang/restic-exporter) for the original work.
> This project is a Go rewrite aimed at fixing outstanding issues (older restic version, fewer metrics, timezone handling, lack of verbose logging).

## Install

### Docker

Docker images are available in [GHCR](https://github.com/adityathebe/restic-exporter/pkgs/container/restic-exporter).

```bash
docker pull ghcr.io/adityathebe/restic-exporter
```

## Configuration

This Prometheus exporter is compatible with all [backends supported by Restic](https://restic.readthedocs.io/en/latest/030_preparing_a_new_repo.html).
Some of them need additional environment variables for the secrets.

All configuration is done with environment variables:

**Restic configuration (required for restic to work)**

| Variable                | Default | Required         | Description                                                                                                                                                              |
| ----------------------- | ------- | ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `RESTIC_REPOSITORY`     | –       | Yes              | Repository URL (e.g. `/data`, `rest:http://user:password@127.0.0.1:8000/`, `s3:s3.amazonaws.com/bucket_name`, `b2:bucketname:path/to/repo`, `rclone:gd-backup:/restic`). |
| `RESTIC_PASSWORD`       | –       | One of           | Repo password (plain text). Required if `RESTIC_PASSWORD_FILE` is not set.                                                                                               |
| `RESTIC_PASSWORD_FILE`  | –       | One of           | Path to a file containing the repo password. Required if `RESTIC_PASSWORD` is not set; mount it into the container.                                                      |
| `AWS_ACCESS_KEY_ID`     | –       | Backend-specific | For Amazon S3 / Minio / Wasabi.                                                                                                                                          |
| `AWS_SECRET_ACCESS_KEY` | –       | Backend-specific | For Amazon S3 / Minio / Wasabi.                                                                                                                                          |
| `B2_ACCOUNT_ID`         | –       | Backend-specific | For Backblaze B2.                                                                                                                                                        |
| `B2_ACCOUNT_KEY`        | –       | Backend-specific | For Backblaze B2.                                                                                                                                                        |

**Exporter behavior (controls how the exporter runs)**

| Variable                         | Default     | Required | Description                                                                                                  |
| -------------------------------- | ----------- | -------- | ------------------------------------------------------------------------------------------------------------ |
| `REFRESH_INTERVAL`               | `600`       | No       | Refresh interval (seconds) for collecting metrics. Higher values reduce load, especially on remote backends. |
| `LISTEN_ADDRESS`                 | `0.0.0.0`   | No       | Bind address for the HTTP server.                                                                            |
| `LISTEN_PORT`                    | `8001`      | No       | Port for the HTTP server.                                                                                    |
| `LOG_LEVEL`                      | `INFO`      | No       | Log level (`DEBUG`, `INFO`, `WARN`, `ERROR`).                                                                |
| `NO_CHECK`                       | empty/false | No       | If set, skip `restic check` for faster scrapes.                                                              |
| `NO_LOCKS`                       | empty/false | No       | If set, skip lock counting.                                                                                  |
| `NO_STATS_SNAPSHOT_RESTORE_SIZE` | empty/false | No       | If set, skip per-snapshot restore size stats for the latest snapshots (`restic stats <snapshot>`);           |
| `NO_STATS_RAW_DATA`              | empty/false | No       | If set, skip repository-wide raw data stats (`restic stats --mode raw-data`) and related metrics.            |
| `INCLUDE_CLIENTS`                | empty/false | No       | Comma-separated list of client hostnames to include; if set, snapshots from other clients are ignored.       |
| `INSECURE_TLS`                   | empty/false | No       | If set, skip TLS verification (self-signed endpoints).                                                       |

Snapshot paths are always included in metrics to make per-path snapshots visible by default.

### Configuration for Rclone

Rclone is not included in the Docker image. You have to mount the Rclone executable and the Rclone configuration from the host machine. Here is an example with docker-compose:

```yaml
version: '2.1'
services:
  restic-exporter:
    image: ngosang/restic-exporter
    container_name: restic-exporter
    environment:
      - TZ=Europe/Madrid
      - RESTIC_REPOSITORY=rclone:gd-backup:/restic
      - RESTIC_PASSWORD=
      - REFRESH_INTERVAL=1800 # 30 min
    volumes:
      - /host_path/restic/data:/data
      - /usr/bin/rclone:/usr/bin/rclone:ro
      - /host_path/restic/rclone.conf:/root/.config/rclone/rclone.conf:ro
    ports:
      - '8001:8001'
    restart: unless-stopped
```

## Exported metrics

```bash
# HELP restic_repository_total_size_bytes Total size of the repository in bytes (raw data)
# TYPE restic_repository_total_size_bytes gauge
restic_repository_total_size_bytes 4.7e+09
# HELP restic_repository_total_uncompressed_size_bytes Total uncompressed size of the repository in bytes
# TYPE restic_repository_total_uncompressed_size_bytes gauge
restic_repository_total_uncompressed_size_bytes 1.1e+10
# HELP restic_compression_ratio Compression ratio of the repository
# TYPE restic_compression_ratio gauge
restic_compression_ratio 2.3
# HELP restic_raw_data_stats_duration_seconds Duration to run the restic stats --mode raw-data command
# TYPE restic_raw_data_stats_duration_seconds gauge
restic_raw_data_stats_duration_seconds 12.4
# HELP restic_snapshots_duration_seconds Duration to run the restic snapshots command
# TYPE restic_snapshots_duration_seconds gauge
restic_snapshots_duration_seconds 4.2
# HELP restic_snapshots_restore_size_duration_seconds Total duration spent collecting restore-size stats for the latest snapshots
# TYPE restic_snapshots_restore_size_duration_seconds gauge
restic_snapshots_restore_size_duration_seconds 8.5
# HELP restic_repository_check_duration_seconds Duration to run the restic check command
# TYPE restic_repository_check_duration_seconds gauge
restic_repository_check_duration_seconds 2.1
# HELP restic_check_success Result of restic check operation in the repository
# TYPE restic_check_success gauge
restic_check_success 1.0
# HELP restic_locks_total Total number of locks in the repository
# TYPE restic_locks_total counter
restic_locks_total 1.0
# HELP restic_snapshots_total Total number of snapshots in the repository
# TYPE restic_snapshots_total counter
restic_snapshots_total 100.0
# HELP restic_backup_timestamp Timestamp of the last backup
# TYPE restic_backup_timestamp gauge
restic_backup_timestamp{client_hostname="product.example.com",client_username="root",client_version="restic 0.16.0",snapshot_hash="20795072cba0953bcdbe52e9cf9d75e5726042f5bbf2584bb2999372398ee835",snapshot_tag="mysql",snapshot_tags="mysql,tag2",snapshot_paths="/mysql/data,/mysql/config"} 1.666273638e+09
# HELP restic_backup_files_total Number of files in the backup
# TYPE restic_backup_files_total counter
restic_backup_files_total{client_hostname="product.example.com",client_username="root",client_version="restic 0.16.0",snapshot_hash="20795072cba0953bcdbe52e9cf9d75e5726042f5bbf2584bb2999372398ee835",snapshot_tag="mysql",snapshot_tags="mysql,tag2",snapshot_paths="/mysql/data,/mysql/config"} 8.0
# HELP restic_backup_size_total Total size of backup in bytes
# TYPE restic_backup_size_total counter
restic_backup_size_total{client_hostname="product.example.com",client_username="root",client_version="restic 0.16.0",snapshot_hash="20795072cba0953bcdbe52e9cf9d75e5726042f5bbf2584bb2999372398ee835",snapshot_tag="mysql",snapshot_tags="mysql,tag2",snapshot_paths="/mysql/data,/mysql/config"} 4.3309562e+07
# HELP restic_backup_snapshots_total Total number of snapshots
# TYPE restic_backup_snapshots_total counter
restic_backup_snapshots_total{client_hostname="product.example.com",client_username="root",client_version="restic 0.16.0",snapshot_hash="20795072cba0953bcdbe52e9cf9d75e5726042f5bbf2584bb2999372398ee835",snapshot_tag="mysql",snapshot_tags="mysql,tag2",snapshot_paths="/mysql/data,/mysql/config"} 1.0
# HELP restic_scrape_duration_seconds Amount of time each scrape takes
# TYPE restic_scrape_duration_seconds gauge
restic_scrape_duration_seconds 166.9411084651947
# HELP restic_last_scrape_timestamp_seconds Unix timestamp of the last metrics scrape
# TYPE restic_last_scrape_timestamp_seconds gauge
restic_last_scrape_timestamp_seconds 1.699e+09
# HELP restic_scrape_success 1 if the last scrape succeeded, 0 otherwise
# TYPE restic_scrape_success gauge
restic_scrape_success 1
```

## Prometheus config

Example Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'restic-exporter'
    static_configs:
      - targets: ['192.168.1.100:8001']
```

## Prometheus / Alertmanager rules

Reference [prometheus_rule.yaml](./monitoring/prometheus_rule.yaml)

## Grafana dashboard

There is a reference Grafana dashboard in [monitoring/grafana_dashboard.json](./monitoring/grafana_dashboard.json).

![](./monitoring/grafana_dashboard.png)

---

Built from: https://github.com/ngosang/restic-exporter
