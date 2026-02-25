package opa

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"testing"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetDesiredUsesDiscovery(t *testing.T) {
	scope := state.Scope{
		SecurityConfig: v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-ns",
			},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "test-app",
			},
		},
		OpaConfig: state.OpaConfig{
			Enabled:   true,
			BundleUrl: "ghcr.io/kartverket/opa-bundle:v1.2.3",
		},
	}

	configMap := GetDesired(
		metav1.ObjectMeta{
			Name:      utilities.GetOpaConfigName("test-app"),
			Namespace: "test-ns",
		},
		scope,
	)
	if assert.NotNil(t, configMap) {
		var cfg OPAConfig
		err := yaml.Unmarshal([]byte(configMap.Data[utilities.OpaConfigFileName]), &cfg)
		assert.NoError(t, err)
		assert.Equal(t, "discovery-server", cfg.Discovery.Service)
		assert.Equal(t, GetOpaDiscoveryResourcePath(), cfg.Discovery.Resource)
		assert.NotContains(t, cfg.Services, "ghcr-registry")
		assert.Nil(t, cfg.Keys)
		assert.Equal(
			t,
			"http://test-app-opa-discovery.test-ns.svc.cluster.local",
			cfg.Services["discovery-server"].URL,
		)
	}
}

func TestGetDiscoveryConfigDesiredUsesInternalBundlePath(t *testing.T) {
	scope := state.Scope{
		SecurityConfig: v1alpha.SecurityConfig{
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "test-app",
				Opa: &v1alpha.OpaSpec{
					Enabled: true,
					GithubToken: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "gh"},
						Key:                  "token",
					},
					BundlePublicKey: corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "pub"},
						Key:                  "key",
					},
				},
			},
		},
		OpaConfig: state.OpaConfig{
			Enabled:   true,
			BundleUrl: "ghcr.io/kartverket/opa-bundle:v2.0.0",
		},
	}

	configMap := GetDiscoveryConfigDesired(
		metav1.ObjectMeta{
			Name:      utilities.GetOpaDiscoveryConfigName("test-app"),
			Namespace: "test-ns",
		},
		scope,
	)
	if assert.NotNil(t, configMap) {
		var discoveryDoc DiscoveryDocument
		discoveryDataJSON, bundleErr := extractDiscoveryDataJSONFromBundle(
			configMap.BinaryData[utilities.OpaDiscoveryBundleFileName],
		)
		assert.NoError(t, bundleErr)
		err := json.Unmarshal([]byte(discoveryDataJSON), &discoveryDoc)
		assert.NoError(t, err)
		assert.Equal(t, "discovery-server", discoveryDoc.Bundles["authz"].Service)
		assert.Equal(t, GetOpaDiscoveryBundleResourcePath(), discoveryDoc.Bundles["authz"].Resource)
	}
}

func TestGetDiscoveryServiceDesiredWhenDisabledReturnsNil(t *testing.T) {
	service := GetDiscoveryServiceDesired(
		metav1.ObjectMeta{Name: "unused", Namespace: "unused"},
		state.Scope{
			OpaConfig: state.OpaConfig{
				Enabled: false,
			},
		},
	)
	assert.Nil(t, service)
}

func TestGetDiscoveryDeploymentDesiredWhenEnabledReturnsExpectedSpec(t *testing.T) {
	scope := state.Scope{
		SecurityConfig: v1alpha.SecurityConfig{
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "test-app",
				Opa: &v1alpha.OpaSpec{
					Enabled: true,
					GithubToken: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "gh"},
						Key:                  "token",
					},
					BundlePublicKey: corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "pub"},
						Key:                  "key",
					},
				},
			},
		},
		OpaConfig: state.OpaConfig{
			Enabled:   true,
			BundleUrl: "ghcr.io/kartverket/opa-bundle:v2.0.0",
		},
	}

	deployment := GetDiscoveryDeploymentDesired(
		metav1.ObjectMeta{
			Name:      utilities.GetOpaDiscoveryDeploymentName("test-app"),
			Namespace: "test-ns",
		},
		scope,
	)
	if assert.NotNil(t, deployment) {
		assert.Equal(t, int32(1), *deployment.Spec.Replicas)
		assert.Equal(t, "nginxinc/nginx-unprivileged:latest", deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Empty(t, deployment.Spec.Template.Spec.InitContainers)
		if assert.Len(t, deployment.Spec.Template.Spec.Containers, 2) {
			fetcher := deployment.Spec.Template.Spec.Containers[1]
			assert.Equal(t, "opa-discovery-bundle-fetcher", fetcher.Name)
			assert.Equal(t, "/opa-discovery-fetcher", fetcher.Command[0])
			assert.Contains(t, fetcher.Args, "-bundle-ref=ghcr.io/kartverket/opa-bundle:v2.0.0")
			assert.Contains(t, fetcher.Args, "-refresh-interval=1m")
		}
		assert.Equal(
			t,
			utilities.GetOpaDiscoveryConfigName("test-app"),
			deployment.Spec.Template.Spec.Volumes[0].ConfigMap.Name,
		)
		assert.Equal(t, "gh", deployment.Spec.Template.Spec.Volumes[1].Secret.SecretName)
		assert.Equal(t, "pub", deployment.Spec.Template.Spec.Volumes[2].ConfigMap.Name)
	}
}

func TestRenderDiscoveryNginxConfEnablesConditionalBundleCaching(t *testing.T) {
	conf := renderDiscoveryNginxConf()

	assert.Contains(t, conf, "etag on;")
	assert.Contains(t, conf, "if_modified_since exact;")
	assert.Contains(t, conf, `location = `+GetOpaDiscoveryBundleResourcePath())
}

func extractDiscoveryDataJSONFromBundle(bundle []byte) (string, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(bundle))
	if err != nil {
		return "", err
	}
	defer func() { _ = gzipReader.Close() }()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			return "", io.EOF
		}
		if nextErr != nil {
			return "", nextErr
		}
		if header.Name == "data.json" {
			content, readErr := io.ReadAll(tarReader)
			if readErr != nil {
				return "", readErr
			}
			return string(content), nil
		}
	}
}
