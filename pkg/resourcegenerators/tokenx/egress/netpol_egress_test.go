package egress

import (
	"testing"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetDesiredIncludesOpaDiscoveryEgressWhenOpaEnabled(t *testing.T) {
	t.Setenv("ACCESSERATOR_CLUSTER_NAME", "test-cluster")
	t.Setenv("ACCESSERATOR_TOKENX_NAMESPACE", "tokenx-namespace")
	t.Setenv("ACCESSERATOR_TEXAS_IMAGE_TAG", "test-tag")
	assert.NoError(t, config.Load())

	scope := state.Scope{
		SecurityConfig: v1alpha.SecurityConfig{
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app",
			},
		},
		TokenXConfig: state.TokenXConfig{
			Enabled: true,
		},
		OpaConfig: state.OpaConfig{
			Enabled: true,
		},
	}

	netpol := GetDesired(metav1.ObjectMeta{Name: "test", Namespace: "test-ns"}, scope)
	if assert.NotNil(t, netpol) {
		assert.Len(t, netpol.Spec.Egress, 2)
		discoverySelector := netpol.Spec.Egress[1].To[0].PodSelector.MatchLabels
		assert.Equal(
			t,
			utilities.GetOpaDiscoveryServiceName("app"),
			discoverySelector["app.kubernetes.io/name"],
		)
		assert.Equal(t, "opa-discovery", discoverySelector["app.kubernetes.io/component"])
	}
}

func TestGetDesiredDoesNotIncludeOpaDiscoveryEgressWhenOpaDisabled(t *testing.T) {
	t.Setenv("ACCESSERATOR_CLUSTER_NAME", "test-cluster")
	t.Setenv("ACCESSERATOR_TOKENX_NAMESPACE", "tokenx-namespace")
	t.Setenv("ACCESSERATOR_TEXAS_IMAGE_TAG", "test-tag")
	assert.NoError(t, config.Load())

	scope := state.Scope{
		SecurityConfig: v1alpha.SecurityConfig{
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: "app",
			},
		},
		TokenXConfig: state.TokenXConfig{
			Enabled: true,
		},
		OpaConfig: state.OpaConfig{
			Enabled: false,
		},
	}

	netpol := GetDesired(metav1.ObjectMeta{Name: "test", Namespace: "test-ns"}, scope)
	if assert.NotNil(t, netpol) {
		assert.Len(t, netpol.Spec.Egress, 1)
	}
}
