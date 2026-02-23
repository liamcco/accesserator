package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func writeHeartbeat(path string) error {
	// Example: writes "1708680000" to `/tmp/fetcher.heartbeat`, which the exec
	// liveness probe later reads to confirm the refresh loop is still advancing.
	if path == "" {
		return nil
	}
	// The fetcher sidecar is not an HTTP server, so liveness is exposed by
	// updating a shared heartbeat file from the refresh loop.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.FormatInt(time.Now().Unix(), 10)), 0o644)
}

func runHealthcheck(heartbeatPath string, maxAge time.Duration) error {
	// Example: if the file contains "1708680000\n" and maxAge is 3m, the check
	// succeeds only while that timestamp is less than 3 minutes old.
	if heartbeatPath == "" {
		return fmt.Errorf("missing healthcheck heartbeat file")
	}
	// This is used by the Kubernetes exec liveness probe to verify that the
	// long-running fetch loop is still making progress, not to test GHCR/network
	// reachability. Transient upstream failures should not fail liveness.
	content, err := os.ReadFile(heartbeatPath)
	if err != nil {
		return err
	}
	sec, err := strconv.ParseInt(string(bytesTrimSpace(content)), 10, 64)
	if err != nil {
		return err
	}
	if maxAge > 0 {
		lastBeat := time.Unix(sec, 0)
		if time.Since(lastBeat) > maxAge {
			return fmt.Errorf("heartbeat too old: %s", time.Since(lastBeat))
		}
	}
	return nil
}

func bytesTrimSpace(b []byte) []byte {
	// Keep this tiny local helper to avoid pulling in bytes.TrimSpace for one
	// probe-only parse path; it returns a subslice without allocating.
	// Example: []byte("  1708680000\n") -> []byte("1708680000"), which lets
	// strconv.ParseInt parse the heartbeat timestamp written by writeHeartbeat.
	i := 0
	j := len(b)
	for i < j && (b[i] == ' ' || b[i] == '\n' || b[i] == '\r' || b[i] == '\t') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\n' || b[j-1] == '\r' || b[j-1] == '\t') {
		j--
	}
	return b[i:j]
}
