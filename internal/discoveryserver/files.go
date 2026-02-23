package discoveryserver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readOptionalTrimmedFile(path string) (string, error) {
	// Mounted token files typically end with a newline; trimming avoids auth
	// failures caused by passing that newline through to registry auth.
	// Example: file contents "ghp_abc123\n" become "ghp_abc123".
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func writeFileAtomically(outputPath string, content []byte, mode os.FileMode) error {
	// nginx may read the mirrored bundle while the fetcher refreshes it. Writing
	// to a temp file and renaming avoids exposing a partially written archive.
	// Example: write `/cache/authz.tar.gz.tmp`, then rename to
	// `/cache/authz.tar.gz` once the full file is on disk.
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	tmpPath := outputPath + ".tmp"
	if err := os.WriteFile(tmpPath, content, mode); err != nil {
		return fmt.Errorf("write temp bundle file: %w", err)
	}
	if err := os.Rename(tmpPath, outputPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("move bundle file into place: %w", err)
	}
	return nil
}

func sha256Hex(content []byte) string {
	// Stored alongside the last remote digest so the refresh loop can detect
	// local corruption even when upstream content has not changed.
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func sha256FileHex(path string) (string, error) {
	// Example: if `/cache/authz.tar.gz` changes on disk, this digest changes even
	// when the remote OCI digest is unchanged, allowing local-corruption repair.
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Hex(content), nil
}
