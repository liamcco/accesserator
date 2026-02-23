package main

import (
	"context"
	"flag"
	"log"
	"os"
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

	flag.StringVar(&bundleRef, "bundle-ref", "", "OCI bundle reference to pull and verify")
	flag.StringVar(&githubTokenFile, "github-token-file", "", "Filesystem path to mounted GitHub token")
	flag.StringVar(&publicKeyFile, "public-key-file", "", "Filesystem path to mounted bundle verification public key")
	flag.StringVar(&expectedKeyID, "expected-key-id", "", "Expected bundle signature key id (optional)")
	flag.StringVar(&outputFile, "output-file", "", "Filesystem path to write the verified bundle archive")
	flag.DurationVar(&refreshInterval, "refresh-interval", 1*time.Minute, "How often to refresh and re-verify the bundle")
	flag.Parse()

	cfg := discoveryserver.Config{
		BundleRef:       bundleRef,
		GithubTokenFile: githubTokenFile,
		PublicKeyFile:   publicKeyFile,
		ExpectedKeyID:   expectedKeyID,
		OutputFile:      outputFile,
	}

	lastDigest, changed, err := discoveryserver.FetchAndVerifyToFileIfChanged(context.Background(), cfg, "")
	if err != nil {
		log.Printf("failed to fetch and verify bundle: %v", err)
		os.Exit(1)
	}
	if changed {
		log.Printf("bundle fetched and verified; digest=%s refresh interval=%s", lastDigest, refreshInterval)
	}

	if refreshInterval <= 0 {
		select {}
	}

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for range ticker.C {
		var changed bool
		lastDigest, changed, err = discoveryserver.FetchAndVerifyToFileIfChanged(context.Background(), cfg, lastDigest)
		if err != nil {
			log.Printf("bundle refresh failed: %v", err)
			continue
		}
		if changed {
			log.Printf("bundle refreshed successfully; digest=%s", lastDigest)
			continue
		}
		log.Printf("bundle unchanged; digest=%s", lastDigest)
	}
}
