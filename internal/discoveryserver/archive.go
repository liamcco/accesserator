package discoveryserver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	// OPA bundles place signature metadata in this file. We verify it server-side
	// and then remove it from the mirrored bundle served to sidecars.
	bundleSignaturesFileName = ".signatures.json"
)

func stripBundleSignatureFile(bundleArchive []byte) ([]byte, error) {
	// Rebuild the tar.gz instead of mutating in place because tar/gzip archives
	// are stream-oriented formats.
	// Example: input entries `["/policy.rego", "/data.json", "/.signatures.json"]`
	// become `["policy.rego", "data.json"]` in the output mirrored bundle.
	gzipReader, err := gzip.NewReader(bytes.NewReader(bundleArchive))
	if err != nil {
		return nil, fmt.Errorf("open bundle gzip: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	var out bytes.Buffer
	gzipWriter := gzip.NewWriter(&out)
	tarReader := tar.NewReader(gzipReader)
	tarWriter := tar.NewWriter(gzipWriter)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, fmt.Errorf("read bundle archive: %w", err)
		}
		if header == nil {
			continue
		}

		if normalizeBundlePath(header.Name) == bundleSignaturesFileName {
			// Signature metadata is removed after server-side verification so OPA
			// sidecars can consume the mirrored bundle without local key config.
			continue
		}

		cloned := *header
		if err := tarWriter.WriteHeader(&cloned); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, fmt.Errorf("write bundle header %s: %w", header.Name, err)
		}

		if header.FileInfo().Mode().IsRegular() {
			if _, err := io.Copy(tarWriter, tarReader); err != nil {
				_ = tarWriter.Close()
				_ = gzipWriter.Close()
				return nil, fmt.Errorf("copy bundle file %s: %w", header.Name, err)
			}
		}
	}

	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close gzip writer: %w", err)
	}
	return out.Bytes(), nil
}

func readBundleArchiveFiles(bundleArchive []byte) (map[string][]byte, []byte, error) {
	// Verification needs random access by normalized file name, so the archive is
	// materialized into memory once and compared against signature claims.
	// Example return:
	// - files["data.json"] = []byte("{\"a\":1}")
	// - signatures     = []byte("{\"signatures\":[...]}") from .signatures.json
	gzipReader, err := gzip.NewReader(bytes.NewReader(bundleArchive))
	if err != nil {
		return nil, nil, fmt.Errorf("open bundle gzip: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	tarReader := tar.NewReader(gzipReader)

	files := map[string][]byte{}
	var signatures []byte
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("read bundle archive: %w", err)
		}
		if header == nil || header.FileInfo().IsDir() {
			continue
		}

		name := normalizeBundlePath(header.Name)
		content, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, nil, fmt.Errorf("read bundle file %s: %w", name, err)
		}

		if name == bundleSignaturesFileName {
			signatures = content
			continue
		}
		files[name] = content
	}

	if len(signatures) == 0 {
		return nil, nil, fmt.Errorf("bundle is missing %s", bundleSignaturesFileName)
	}
	return files, signatures, nil
}

func normalizeBundlePath(p string) string {
	// Signature claims and tar headers may differ in leading "./" or "/" usage.
	// Normalize both sides before comparing file names.
	// Examples:
	// - "./policy/authz.rego" -> "policy/authz.rego"
	// - "/data.json"          -> "data.json"
	clean := path.Clean(strings.TrimSpace(p))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")
	return clean
}

func canonicalizeBundleFileForHash(name string, content []byte) ([]byte, error) {
	// OPA signs structured files by content semantics rather than raw bytes, so
	// JSON/YAML/.manifest are normalized before hashing. This avoids false hash
	// mismatches from formatting differences (spacing/key order in source YAML).
	// Example: `{"b":2,"a":1}` and `a: 1\nb: 2\n` both canonicalize to the same
	// JSON bytes (`{"a":1,"b":2}`) before SHA-256 is computed.
	if !isStructuredBundleFile(name) {
		return content, nil
	}

	var jsonValue any
	switch {
	case strings.HasSuffix(name, ".json") || name == ".manifest":
		if err := json.Unmarshal(content, &jsonValue); err != nil {
			return nil, err
		}
	case strings.HasSuffix(name, ".yaml"), strings.HasSuffix(name, ".yml"):
		jsonBytes, err := yaml.YAMLToJSON(content)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(jsonBytes, &jsonValue); err != nil {
			return nil, err
		}
	default:
		return content, nil
	}

	// encoding/json emits deterministic object-key ordering, which matches the
	// canonicalized structured-file hashing OPA describes for bundle signatures.
	return json.Marshal(jsonValue)
}

func isStructuredBundleFile(name string) bool {
	// Keep the file-type rule in one place so hash verification and future tests
	// stay aligned with the canonicalization policy above.
	return name == ".manifest" ||
		strings.HasSuffix(name, ".json") ||
		strings.HasSuffix(name, ".yaml") ||
		strings.HasSuffix(name, ".yml")
}
