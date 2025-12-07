# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Prometheus exporter for Restic backup systems, written in Go. It periodically queries a Restic repository and exposes backup metrics (snapshot counts, sizes, timestamps, repository health) via HTTP for Prometheus to scrape.

## Development Commands

### Build

```bash
make build              # Build binary to bin/restic-exporter
```

### Test

```bash
make test               # Run all tests with verbose output (go test -v ./...)
```

## Notes

- don't run go build and leave the build artifacts on the repository
