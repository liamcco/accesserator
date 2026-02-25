package opa

import (
	"context"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const maxConfigMapBinaryDataSize = 1024 * 1024

var BundleFetcher = FetchBundle

func FetchBundle(ctx context.Context, bundleReference, token string) ([]byte, error) {
	bundle, err := fetchOCIBundleArchive(ctx, bundleReference, token)
	if err != nil {
		return nil, err
	}
	if len(bundle) == 0 {
		return nil, fmt.Errorf("OPA bundle %q is empty", bundleReference)
	}
	if len(bundle) > maxConfigMapBinaryDataSize {
		return nil, fmt.Errorf(
			"OPA bundle %q is %d bytes and exceeds ConfigMap limit (%d bytes)",
			bundleReference,
			len(bundle),
			maxConfigMapBinaryDataSize,
		)
	}

	return bundle, nil
}

func fetchOCIBundleArchive(ctx context.Context, bundleRef, githubToken string) ([]byte, error) {
	// This fetches the actual compressed bundle bytes from the OCI artifact layer.
	// It is only called when the refresh path decides a rewrite is needed.
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

func selectBundleLayer(img v1.Image) (v1.Layer, error) {
	// Current bundles are single-layer OCI artifacts. We use the last layer to be
	// tolerant if metadata layers are added ahead of the payload.
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("list bundle layers: %w", err)
	}
	if len(layers) == 0 {
		return nil, fmt.Errorf("bundle image has no layers")
	}

	return layers[len(layers)-1], nil
}

func remoteOptions(ctx context.Context, githubToken string) []remote.Option {
	options := []remote.Option{remote.WithContext(ctx)}
	if githubToken != "" {
		return append(options, remote.WithAuth(&authn.Basic{
			Username: "oauth2",
			Password: githubToken,
		}))
	}
	return append(options, remote.WithAuth(authn.Anonymous))
}
