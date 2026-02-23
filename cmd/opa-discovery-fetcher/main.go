package main

import (
	"flag"
	"log"
	"os"
	"time"
)

func main() {
	// Example normal mode invocation (sidecar):
	// /opa-discovery-fetcher \
	//   -bundle-ref=ghcr.io/acme/opa-bundle:latest \
	//   -github-token-file=/var/run/accesserator/opa-secret/github-token \
	//   -public-key-file=/var/run/accesserator/opa-public-key/public_sign_key \
	//   -output-file=/usr/share/nginx/html/bundles/authz.tar.gz \
	//   -refresh-interval=1m
	//
	// Example probe mode invocation (exec livenessProbe):
	// /opa-discovery-fetcher -healthcheck-heartbeat-file=/tmp/fetcher.heartbeat -healthcheck-max-age=3m
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
		// The same binary is used both as the long-running fetcher sidecar and as
		// the Kubernetes exec liveness probe command. Supplying the probe flag
		// switches to a short-lived "check and exit" mode.
		if err := runHealthcheck(healthcheckHeartbeatFile, healthcheckMaxAge); err != nil {
			log.Printf("healthcheck failed: %v", err)
			os.Exit(1)
		}
		return
	}

	// Normal mode: start the refresh loop that fetches, verifies, and rewrites
	// the mirrored bundle for nginx to serve.
	if err := runFetcher(fetcherRunConfig{
		bundleRef:       bundleRef,
		githubTokenFile: githubTokenFile,
		publicKeyFile:   publicKeyFile,
		expectedKeyID:   expectedKeyID,
		outputFile:      outputFile,
		refreshInterval: refreshInterval,
		heartbeatFile:   heartbeatFile,
	}); err != nil {
		log.Printf("failed to fetch and verify bundle: %v", err)
		os.Exit(1)
	}
}
