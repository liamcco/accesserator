package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kartverket/accesserator/internal/discoveryserver"
)

type fetcherRunConfig struct {
	bundleRef       string
	githubTokenFile string
	publicKeyFile   string
	expectedKeyID   string
	outputFile      string
	refreshInterval time.Duration
	heartbeatFile   string
}

func runFetcher(cfg fetcherRunConfig) error {
	// Example lifecycle:
	// - startup: fetch+verify+write `/usr/share/nginx/html/bundles/authz.tar.gz`
	// - every 1m: compare remote OCI digest and local checksum
	// - only re-download when upstream changed or the local file is missing/corrupt
	discoveryCfg := discoveryserver.Config{
		BundleRef:       cfg.bundleRef,
		GithubTokenFile: cfg.githubTokenFile,
		PublicKeyFile:   cfg.publicKeyFile,
		ExpectedKeyID:   cfg.expectedKeyID,
		OutputFile:      cfg.outputFile,
	}

	// Mark the process as alive before the first fetch attempt so liveness does
	// not flap during slow startup/network calls.
	_ = writeHeartbeat(cfg.heartbeatFile)
	lastDigest, lastLocalChecksum, changed, err := discoveryserver.FetchAndVerifyToFileIfChanged(
		context.Background(),
		discoveryCfg,
		"",
		"",
	)
	if err != nil {
		return err
	}
	if changed {
		log.Printf("bundle fetched and verified; digest=%s refresh interval=%s", lastDigest, cfg.refreshInterval)
	}
	// A second heartbeat after startup fetch confirms the loop is still making
	// progress even if the first fetch was long-running.
	_ = writeHeartbeat(cfg.heartbeatFile)

	if cfg.refreshInterval <= 0 {
		// "Fetch once" mode: keep the container alive so nginx can continue serving
		// the mirrored bundle written above.
		select {}
	}

	ticker := time.NewTicker(cfg.refreshInterval)
	defer ticker.Stop()
	for range ticker.C {
		// Heartbeat is updated before each iteration so the liveness probe tracks
		// loop progression, not only successful refreshes.
		_ = writeHeartbeat(cfg.heartbeatFile)
		var changed bool
		lastDigest, lastLocalChecksum, changed, err = discoveryserver.FetchAndVerifyToFileIfChanged(
			context.Background(),
			discoveryCfg,
			lastDigest,
			lastLocalChecksum,
		)
		if err != nil {
			log.Printf("bundle refresh failed: %v", err)
			continue
		}
		if changed {
			// "changed" means the local served file was rewritten. That can happen
			// because the upstream digest changed or because local corruption/missing
			// file was detected and repaired while upstream stayed the same.
			log.Printf("bundle refreshed successfully; digest=%s", lastDigest)
			_ = writeHeartbeat(cfg.heartbeatFile)
			continue
		}
		log.Printf("bundle unchanged; digest=%s", lastDigest)
		_ = writeHeartbeat(cfg.heartbeatFile)
	}

	return fmt.Errorf("fetcher loop exited unexpectedly")
}
