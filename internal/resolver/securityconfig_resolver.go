package resolver

import (
	"context"
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/skiperator/api/v1alpha1"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ResolveSecurityConfig(ctx context.Context, k8sClient client.Client, securityConfig v1alpha.SecurityConfig) (*state.Scope, error) {
	tokenXEnabled := securityConfig.Spec.Tokenx != nil && securityConfig.Spec.Tokenx.Enabled
	opaConfigEnabled := securityConfig.Spec.Opa != nil && securityConfig.Spec.Opa.Enabled
	bundleUrl := securityConfig.Spec.Opa.BundlePath + ":" + securityConfig.Spec.Opa.BundleVersion

	if !tokenXEnabled {
		return &state.Scope{
			SecurityConfig: securityConfig,
			TokenXConfig: state.TokenXConfig{
				Enabled: tokenXEnabled,
			},
			OpaConfig: state.OpaConfig{
				Enabled:   opaConfigEnabled,
				BundleUrl: bundleUrl,
			},
		}, nil
	}

	var skiperatorApplication v1alpha1.Application
	if exists := k8sClient.Get(ctx, types.NamespacedName{
		Name:      securityConfig.Spec.ApplicationRef,
		Namespace: securityConfig.Namespace,
	}, &skiperatorApplication); exists != nil {
		return nil, fmt.Errorf(
			"failed to fetch Application resource named %s: %w",
			securityConfig.Spec.ApplicationRef,
			exists,
		)
	}

	var skiperatorAccessPolicy *podtypes.AccessPolicy
	if skiperatorApplication.Spec.AccessPolicy != nil {
		skiperatorAccessPolicy = skiperatorApplication.Spec.AccessPolicy
	} else {
		skiperatorAccessPolicy = nil
	}

	return &state.Scope{
		SecurityConfig: securityConfig,
		TokenXConfig: state.TokenXConfig{
			Enabled:      tokenXEnabled,
			AccessPolicy: skiperatorAccessPolicy,
		},
		OpaConfig: state.OpaConfig{
			Enabled:   opaConfigEnabled,
			BundleUrl: bundleUrl,
		},
	}, nil
}
