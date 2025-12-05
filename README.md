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

| Variable           | Default     | Required | Description                                                                                                  |
| ------------------ | ----------- | -------- | ------------------------------------------------------------------------------------------------------------ |
| `REFRESH_INTERVAL` | `600`       | No       | Refresh interval (seconds) for collecting metrics. Higher values reduce load, especially on remote backends. |
| `LISTEN_ADDRESS`   | `0.0.0.0`   | No       | Bind address for the HTTP server.                                                                            |
| `LISTEN_PORT`      | `8001`      | No       | Port for the HTTP server.                                                                                    |
| `LOG_LEVEL`        | `INFO`      | No       | Log level (`DEBUG`, `INFO`, `WARN`, `ERROR`).                                                                |
| `NO_CHECK`         | empty/false | No       | If set, skip `restic check` for faster scrapes.                                                              |
| `NO_STATS`         | empty/false | No       | If set, skip collecting per-snapshot stats from `restic stats` (size and file counts).                       |
| `NO_LOCKS`         | empty/false | No       | If set, skip lock counting.                                                                                  |
| `INCLUDE_PATHS`    | empty/false | No       | If set, include snapshot paths in metrics (comma-separated).                                                 |
| `INSECURE_TLS`     | empty/false | No       | If set, skip TLS verification (self-signed endpoints).                                                       |

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

Example Prometheus rules for alerting:

```yaml
- alert: ResticCheckFailed
  expr: restic_check_success == 0
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: Restic check failed (instance {{ $labels.instance }})
    description: Restic check failed\n  VALUE = {{ $value }}\n  LABELS = {{ $labels }}

- alert: ResticOutdatedBackup
  # 1209600 = 15 days
  expr: time() - restic_backup_timestamp > 1209600
  for: 0m
  labels:
    severity: critical
  annotations:
    summary: Restic {{ $labels.client_hostname }} / {{ $labels.client_username }} backup is outdated
    description: Restic backup is outdated\n  VALUE = {{ $value }}\n  LABELS = {{ $labels }}
```

## Grafana dashboard

There is a reference Grafana dashboard in [grafana/grafana_dashboard.json](./grafana/grafana_dashboard.json).

![](./grafana/grafana_dashboard.png)

---

Built from: https://github.com/ngosang/restic-exporter
