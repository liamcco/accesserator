package opa

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
)

type DiscoveryDocument struct {
	Bundles map[string]Bundle `json:"bundles"`
}

func buildDiscoveryBundleArchive(discoveryDocument DiscoveryDocument) ([]byte, error) {
	// Discovery expects an OPA bundle archive (tar.gz). The bundle data document
	// contains dynamic bundle configuration that OPA merges into runtime config.
	discoveryDataJSON, err := json.Marshal(discoveryDocument)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "data.json",
		Mode: 0o644,
		Size: int64(len(discoveryDataJSON)),
	}); err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return nil, err
	}

	if _, err := tarWriter.Write(discoveryDataJSON); err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return nil, err
	}

	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
