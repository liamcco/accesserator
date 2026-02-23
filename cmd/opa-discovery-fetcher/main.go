package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/kartverket/accesserator/internal/discoveryserver"
)

func main() {
	var bundleRef string
	var githubTokenFile string
	var publicKeyFile string
	var expectedKeyID string
	var outputFile string
	var refreshInterval time.Duration
	var heartbeatFile string
	var healthcheckHeartbeatFile string
	var healthcheckMaxAge time.Duration

	flag.StringVar(&bundleRef, "bundle-ref", "", "OCI bundle reference to pull and verify")
	flag.StringVar(&githubTokenFile, "github-token-file", "", "Filesystem path to mounted GitHub token")
	flag.StringVar(&publicKeyFile, "public-key-file", "", "Filesystem path to mounted bundle verification public key")
	flag.StringVar(&expectedKeyID, "expected-key-id", "", "Expected bundle signature key id (optional)")
	flag.StringVar(&outputFile, "output-file", "", "Filesystem path to write the verified bundle archive")
	flag.DurationVar(&refreshInterval, "refresh-interval", 1*time.Minute, "How often to refresh and re-verify the bundle")
	flag.StringVar(&heartbeatFile, "heartbeat-file", "", "Filesystem path to write fetcher heartbeat timestamps")
	flag.StringVar(&healthcheckHeartbeatFile, "healthcheck-heartbeat-file", "", "Probe mode: heartbeat file to verify")
	flag.DurationVar(&healthcheckMaxAge, "healthcheck-max-age", 0, "Probe mode: max heartbeat age before failing")
	flag.Parse()

	if healthcheckHeartbeatFile != "" {
		if err := runHealthcheck(healthcheckHeartbeatFile, healthcheckMaxAge); err != nil {
			log.Printf("healthcheck failed: %v", err)
			os.Exit(1)
		}
		return
	}

	cfg := discoveryserver.Config{
		BundleRef:       bundleRef,
		GithubTokenFile: githubTokenFile,
		PublicKeyFile:   publicKeyFile,
		ExpectedKeyID:   expectedKeyID,
		OutputFile:      outputFile,
	}

	_ = writeHeartbeat(heartbeatFile)
	lastDigest, lastLocalChecksum, changed, err := discoveryserver.FetchAndVerifyToFileIfChanged(
		context.Background(),
		cfg,
		"",
		"",
	)
	if err != nil {
		log.Printf("failed to fetch and verify bundle: %v", err)
		os.Exit(1)
	}
	if changed {
		log.Printf("bundle fetched and verified; digest=%s refresh interval=%s", lastDigest, refreshInterval)
	}
	_ = writeHeartbeat(heartbeatFile)

	if refreshInterval <= 0 {
		select {}
	}

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for range ticker.C {
		_ = writeHeartbeat(heartbeatFile)
		var changed bool
		lastDigest, lastLocalChecksum, changed, err = discoveryserver.FetchAndVerifyToFileIfChanged(
			context.Background(),
			cfg,
			lastDigest,
			lastLocalChecksum,
		)
		if err != nil {
			log.Printf("bundle refresh failed: %v", err)
			continue
		}
		if changed {
			log.Printf("bundle refreshed successfully; digest=%s", lastDigest)
			_ = writeHeartbeat(heartbeatFile)
			continue
		}
		log.Printf("bundle unchanged; digest=%s", lastDigest)
		_ = writeHeartbeat(heartbeatFile)
	}
}

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
