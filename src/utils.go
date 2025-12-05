package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func formatCommandError(err error, stderr string) string {
	exitCode := 1
	if ee, ok := err.(*exec.ExitError); ok {
		if status, ok := ee.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
		}
	}

	output := strings.TrimSpace(stderr)
	if output == "" {
		output = err.Error()
	}

	return fmt.Sprintf("%s Exit code: %d", strings.ReplaceAll(output, "\n", " "), exitCode)
}

func calcSnapshotHash(snap snapshot) string {
	text := snap.Hostname + snap.Username + strings.Join(snap.Paths, ",")
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

func parseResticTime(raw string) (float64, error) {
	trimmed := stripFractionalSecond(raw)

	var ts time.Time
	var err error
	if len(trimmed) > 19 {
		ts, err = time.Parse("2006-01-02T15:04:05-07:00", trimmed)
	} else {
		ts, err = time.ParseInLocation("2006-01-02T15:04:05", trimmed, time.Local)
	}
	if err != nil {
		return 0, err
	}

	return float64(ts.Unix()), nil
}

func stripFractionalSecond(value string) string {
	idx := strings.IndexByte(value, '.')
	if idx == -1 {
		return value
	}

	rest := value[idx+1:]
	end := len(rest)
	for i := 0; i < len(rest); i++ {
		if rest[i] == '+' || rest[i] == '-' {
			end = i
			break
		}
	}

	return value[:idx] + rest[end:]
}

func firstTag(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return tags[0]
}

func snapshotPaths(includePaths bool, paths []string) string {
	if !includePaths {
		return ""
	}
	return strings.Join(paths, ",")
}

func envBool(name string) bool {
	val, ok := os.LookupEnv(name)
	if !ok {
		return false
	}
	b, err := strconv.ParseBool(strings.TrimSpace(val))
	if err != nil {
		return false
	}
	return b
}

func envInt(name string, defaultVal int) int {
	val := strings.TrimSpace(os.Getenv(name))
	if val == "" {
		return defaultVal
	}

	num, err := strconv.Atoi(val)
	if err != nil {
		logger.Warn("Invalid integer env, using default", "name", name, "value", val, "default", defaultVal)
		return defaultVal
	}

	return num
}

func envCSV(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}

	return values
}
