package discoveryserver

import (
	"context"
	"fmt"
	"os"
)

type Config struct {
	// BundleRef is the upstream OCI artifact reference (GHCR in current usage).
	BundleRef string
	// GithubTokenFile and PublicKeyFile are mounted into the discovery pod.
	// Reading them here keeps secret/key handling inside the runtime component
	// instead of the controller.
	GithubTokenFile string
	PublicKeyFile   string
	// ExpectedKeyID is optional. When set, the verifier enforces JWS/payload key
	// metadata in addition to cryptographic verification.
	ExpectedKeyID string
	// OutputFile is the nginx-served mirrored bundle path.
	OutputFile string
}

func FetchAndVerifyToFile(ctx context.Context, cfg Config) error {
	// Example: used on first startup when there is no previously known digest or
	// local checksum yet; this forces one full fetch+verify+write cycle.
	_, _, _, err := FetchAndVerifyToFileIfChanged(ctx, cfg, "", "")
	return err
}

func FetchAndVerifyToFileIfChanged(
	ctx context.Context,
	cfg Config,
	lastRemoteDigest string,
	lastLocalChecksum string,
) (string, string, bool, error) {
	// This is the fetcher's core "smart refresh" path:
	// 1. check remote OCI digest (cheap),
	// 2. skip full download if digest and local checksum still match,
	// 3. otherwise pull, verify, strip signatures, and rewrite atomically.
	// Example:
	// - lastRemoteDigest="sha256:aaa", lastLocalChecksum="111"
	// - remote digest still "sha256:aaa" and output file hashes to "111"
	// => returns (same digest, same checksum, changed=false, nil) without pull.
	// If the output file is missing/corrupt, the same digest still triggers a
	// re-download and repair (changed=true).
	if err := validateConfig(cfg); err != nil {
		return lastRemoteDigest, lastLocalChecksum, false, err
	}

	token, err := readOptionalTrimmedFile(cfg.GithubTokenFile)
	if err != nil {
		return lastRemoteDigest, lastLocalChecksum, false, fmt.Errorf("read github token: %w", err)
	}

	remoteDigest, err := fetchOCIBundleRemoteDigest(ctx, cfg.BundleRef, token)
	if err != nil {
		return lastRemoteDigest, lastLocalChecksum, false, err
	}
	if remoteDigest == lastRemoteDigest {
		// A matching remote digest is not enough: we also verify the local served
		// file checksum so local corruption/deletion self-heals on the next tick.
		localChecksum, checksumErr := sha256FileHex(cfg.OutputFile)
		if checksumErr == nil && localChecksum == lastLocalChecksum && localChecksum != "" {
			return remoteDigest, localChecksum, false, nil
		}
	}

	publicKeyPEM, err := os.ReadFile(cfg.PublicKeyFile)
	if err != nil {
		return lastRemoteDigest, lastLocalChecksum, false, fmt.Errorf("read public key: %w", err)
	}

	mirroredBundle, err := fetchOCIBundleArchive(ctx, cfg.BundleRef, token)
	if err != nil {
		return lastRemoteDigest, lastLocalChecksum, false, err
	}
	if err := verifyBundleArchiveSignature(mirroredBundle, publicKeyPEM, cfg.ExpectedKeyID); err != nil {
		return lastRemoteDigest, lastLocalChecksum, false, err
	}

	// The discovery server verifies signatures once, then serves an unsigned
	// bundle to OPA sidecars so they do not need key/signing configuration.
	unsignedBundle, err := stripBundleSignatureFile(mirroredBundle)
	if err != nil {
		return lastRemoteDigest, lastLocalChecksum, false, err
	}
	localChecksum := sha256Hex(unsignedBundle)

	if err := writeFileAtomically(cfg.OutputFile, unsignedBundle, 0o644); err != nil {
		return lastRemoteDigest, lastLocalChecksum, false, err
	}

	return remoteDigest, localChecksum, true, nil
}

func validateConfig(cfg Config) error {
	switch {
	case cfg.BundleRef == "":
		return fmt.Errorf("missing bundle ref")
	case cfg.GithubTokenFile == "":
		return fmt.Errorf("missing github token file")
	case cfg.PublicKeyFile == "":
		return fmt.Errorf("missing public key file")
	case cfg.OutputFile == "":
		return fmt.Errorf("missing output file")
	default:
		return nil
	}
}
