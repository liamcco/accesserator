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
	discoveryCfg := discoveryserver.Config{
		BundleRef:       cfg.bundleRef,
		GithubTokenFile: cfg.githubTokenFile,
		PublicKeyFile:   cfg.publicKeyFile,
		ExpectedKeyID:   cfg.expectedKeyID,
		OutputFile:      cfg.outputFile,
	}

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
	_ = writeHeartbeat(cfg.heartbeatFile)

	if cfg.refreshInterval <= 0 {
		select {}
	}

	ticker := time.NewTicker(cfg.refreshInterval)
	defer ticker.Stop()
	for range ticker.C {
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
			log.Printf("bundle refreshed successfully; digest=%s", lastDigest)
			_ = writeHeartbeat(cfg.heartbeatFile)
			continue
		}
		log.Printf("bundle unchanged; digest=%s", lastDigest)
		_ = writeHeartbeat(cfg.heartbeatFile)
	}

	return fmt.Errorf("fetcher loop exited unexpectedly")
}
