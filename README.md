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

#### Supported Architectures

The architectures supported by this image are:

- linux/386
- linux/amd64
- linux/arm/v6
- linux/arm/v7
- linux/arm64/v8
- linux/ppc64le
- linux/s390x

#### docker-compose

Compatible with docker-compose v2 schemas:

```yaml
services:
  restic-exporter:
    build: .
    image: restic-exporter:go
    environment:
      - RESTIC_REPOSITORY=/data
      - RESTIC_PASSWORD=<password_here>
      # - RESTIC_PASSWORD_FILE=</file_with_password_here>
      - REFRESH_INTERVAL=1800 # 30 min
    volumes:
      - /host_path/restic/data:/data
    ports:
      - '8001:8001'
    restart: unless-stopped
```

#### docker cli

```bash
docker run -d \
  --name=restic-exporter \
  -e TZ=Europe/Madrid \
  -e RESTIC_REPOSITORY=/data \
  -e RESTIC_PASSWORD=<password_here> \
  -e REFRESH_INTERVAL=1800 \
  -p 8001:8001 \
  --restart unless-stopped \
  ngosang/restic-exporter
```

## Configuration

This Prometheus exporter is compatible with all [backends supported by Restic](https://restic.readthedocs.io/en/latest/030_preparing_a_new_repo.html).
Some of them need additional environment variables for the secrets.

All configuration is done with environment variables:

- `RESTIC_REPOSITORY`: Restic repository URL. All backends are supported. Examples:

  - Local repository: `/data`
  - REST Server: `rest:http://user:password@127.0.0.1:8000/`
  - Amazon S3: `s3:s3.amazonaws.com/bucket_name`
  - Backblaze B2: `b2:bucketname:path/to/repo`
  - Rclone (see notes below): `rclone:gd-backup:/restic`

- `RESTIC_PASSWORD`: Restic repository password in plain text. This is only
  required if `RESTIC_PASSWORD_FILE` is not defined.
- `RESTIC_PASSWORD_FILE`: File with the Restic repository password in plain
  text. This is only required if `RESTIC_PASSWORD` is not defined. Remember
  to mount the Docker volume with the file.
- `AWS_ACCESS_KEY_ID`: (Optional) Required for Amazon S3, Minio and Wasabi
  backends.
- `AWS_SECRET_ACCESS_KEY`: (Optional) Required for Amazon S3, Minio and Wasabi
  backends.
- `B2_ACCOUNT_ID`: (Optional) Required for Backblaze B2 backend.
- `B2_ACCOUNT_KEY`: (Optional) Required for Backblaze B2 backend.
- `REFRESH_INTERVAL`: (Optional) Refresh interval for the metrics in seconds.
  Computing the metrics is an expensive task, keep this value as high as possible.
  Default is `60` seconds.
  - **WARNING**: With default settings, downloading from remote repositories
    may be costly if using this exporter with a remote Cloud-based restic repository
    (e.g. GCP GCS, Amazon S3). This may cause a surprisingly high spike in your
    infrastructure costs (e.g. for small restic repositories that don't download
    frequently, this may increase your costs by multiple orders of magnitude).
    Consider setting `REFRESH_INTERVAL` to considerably higher values (e.g. `86400`
    for once per day) to lower this impact.
- `LISTEN_PORT`: (Optional) The address the exporter should listen on. The
  default is `8001`.
- `LISTEN_ADDRESS`: (Optional) The address the exporter should listen on. The
  default is to listen on all addresses.
- `LOG_LEVEL`: (Optional) Log level of the traces. The default is `INFO`.
- `NO_CHECK`: (Optional) Do not perform `restic check` operation for performance
  reasons. Default is `False` (perform `restic check`).
- `NO_STATS`: (Optional) Do not collect per backup statistics for performance
  reasons. Default is `False` (collect per backup statistics).
- `NO_LOCKS`: (Optional) Do not collect the number of locks. Default is `False` (collect the number of locks).
- `INCLUDE_PATHS`: (Optional) Include snapshot paths for each backup. The paths are separated by commas. Default is `False` (not collect the paths).
- `INSECURE_TLS`: (Optional) skip TLS verification for self-signed certificates. Default is `False` (not skip).

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
