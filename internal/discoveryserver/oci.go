package discoveryserver

import (
	"context"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func fetchOCIBundleArchive(ctx context.Context, bundleRef, githubToken string) ([]byte, error) {
	// This fetches the actual compressed bundle bytes from the OCI artifact layer.
	// It is only called when the refresh path decides a rewrite is needed.
	// Example: for `ghcr.io/acme/opa-bundle:v1`, this returns the tar.gz layer
	// bytes that OPA would normally download as the bundle payload.
	ref, err := name.ParseReference(bundleRef)
	if err != nil {
		return nil, fmt.Errorf("parse bundle ref: %w", err)
	}

	options := remoteOptions(ctx, githubToken)

	img, err := remote.Image(ref, options...)
	if err != nil {
		return nil, fmt.Errorf("pull bundle image: %w", err)
	}

	layer, err := selectBundleLayer(img)
	if err != nil {
		return nil, err
	}
	reader, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("open compressed bundle layer: %w", err)
	}
	defer func() { _ = reader.Close() }()

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read compressed bundle layer: %w", err)
	}
	return content, nil
}

func fetchOCIBundleRemoteDigest(ctx context.Context, bundleRef, githubToken string) (string, error) {
	// Cheap change detection: reading the descriptor/manifest digest avoids
	// downloading the bundle layer on every refresh tick.
	// Example: returns `sha256:7f...` for `ghcr.io/acme/opa-bundle:latest`; if the
	// tag still points to the same manifest on the next tick, we skip layer pull.
	ref, err := name.ParseReference(bundleRef)
	if err != nil {
		return "", fmt.Errorf("parse bundle ref: %w", err)
	}

	desc, err := remote.Get(ref, remoteOptions(ctx, githubToken)...)
	if err != nil {
		return "", fmt.Errorf("read bundle descriptor: %w", err)
	}
	return desc.Digest.String(), nil
}

func remoteOptions(ctx context.Context, githubToken string) []remote.Option {
	// Example:
	// - token present   -> basic auth (`oauth2` + PAT) for private GHCR bundle
	// - token absent    -> anonymous pulls for public bundle refs
	options := []remote.Option{remote.WithContext(ctx)}
	if githubToken != "" {
		// GHCR PATs are commonly authenticated via basic auth (PAT as password).
		return append(options, remote.WithAuth(&authn.Basic{
			Username: "oauth2",
			Password: githubToken,
		}))
	}
	return append(options, remote.WithAuth(authn.Anonymous))
}

func selectBundleLayer(img v1.Image) (v1.Layer, error) {
	// Current bundles are single-layer OCI artifacts. We use the last layer to be
	// tolerant if metadata layers are added ahead of the payload.
	// Example: image layers [config-metadata, bundle.tar.gz] -> choose the last
	// layer so the fetcher reads the actual bundle payload.
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("list bundle layers: %w", err)
	}
	if len(layers) == 0 {
		return nil, fmt.Errorf("bundle image has no layers")
	}
	return layers[len(layers)-1], nil
}
