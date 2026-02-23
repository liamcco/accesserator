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
	options := []remote.Option{remote.WithContext(ctx)}
	if githubToken != "" {
		return append(options, remote.WithAuth(&authn.Basic{
			Username: "oauth2",
			Password: githubToken,
		}))
	}
	return append(options, remote.WithAuth(authn.Anonymous))
}

func selectBundleLayer(img v1.Image) (v1.Layer, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("list bundle layers: %w", err)
	}
	if len(layers) == 0 {
		return nil, fmt.Errorf("bundle image has no layers")
	}
	return layers[len(layers)-1], nil
}
