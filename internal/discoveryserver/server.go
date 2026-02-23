package discoveryserver

import (
	"context"
	"fmt"
	"os"
)

type Config struct {
	BundleRef       string
	GithubTokenFile string
	PublicKeyFile   string
	ExpectedKeyID   string
	OutputFile      string
}

func FetchAndVerifyToFile(ctx context.Context, cfg Config) error {
	_, _, _, err := FetchAndVerifyToFileIfChanged(ctx, cfg, "", "")
	return err
}

func FetchAndVerifyToFileIfChanged(
	ctx context.Context,
	cfg Config,
	lastRemoteDigest string,
	lastLocalChecksum string,
) (string, string, bool, error) {
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
