package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func writeHeartbeat(path string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.FormatInt(time.Now().Unix(), 10)), 0o644)
}

func runHealthcheck(heartbeatPath string, maxAge time.Duration) error {
	if heartbeatPath == "" {
		return fmt.Errorf("missing healthcheck heartbeat file")
	}
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
