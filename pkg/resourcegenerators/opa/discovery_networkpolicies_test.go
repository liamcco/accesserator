package opa

import (
	"testing"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetDiscoveryEgressNetworkPolicyDesiredWhenEnabled(t *testing.T) {
	scope := state.Scope{
		SecurityConfig: v1alpha.SecurityConfig{
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app",
			},
		},
		OpaConfig: state.OpaConfig{
			Enabled: true,
		},
	}

	np := GetDiscoveryEgressNetworkPolicyDesired(
		metav1.ObjectMeta{
			Name:      utilities.GetOpaDiscoveryEgressPolicyName("app"),
			Namespace: "test",
		},
		scope,
	)
	if assert.NotNil(t, np) {
		assert.Equal(t, "app", np.Spec.PodSelector.MatchLabels["app"])
		assert.Equal(
			t,
			utilities.GetOpaDiscoveryServiceName("app"),
			np.Spec.Egress[0].To[0].PodSelector.MatchLabels["app.kubernetes.io/name"],
		)
	}
}

func TestGetDiscoveryIngressNetworkPolicyDesiredWhenEnabled(t *testing.T) {
	scope := state.Scope{
		SecurityConfig: v1alpha.SecurityConfig{
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app",
			},
		},
		OpaConfig: state.OpaConfig{
			Enabled: true,
		},
	}

	np := GetDiscoveryIngressNetworkPolicyDesired(
		metav1.ObjectMeta{
			Name:      utilities.GetOpaDiscoveryIngressPolicyName("app"),
			Namespace: "test",
		},
		scope,
	)
	if assert.NotNil(t, np) {
		assert.Equal(
			t,
			utilities.GetOpaDiscoveryServiceName("app"),
			np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"],
		)
		assert.Equal(t, "app", np.Spec.Ingress[0].From[0].PodSelector.MatchLabels["app"])
	}
}

func TestGetDiscoveryNetworkPoliciesWhenDisabledReturnNil(t *testing.T) {
	scope := state.Scope{
		SecurityConfig: v1alpha.SecurityConfig{
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app",
			},
		},
		OpaConfig: state.OpaConfig{
			Enabled: false,
		},
	}

	assert.Nil(t, GetDiscoveryEgressNetworkPolicyDesired(metav1.ObjectMeta{}, scope))
	assert.Nil(t, GetDiscoveryIngressNetworkPolicyDesired(metav1.ObjectMeta{}, scope))
}
