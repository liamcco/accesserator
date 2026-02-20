package egress

import (
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(objectMeta metav1.ObjectMeta, scope state.Scope) *v1.NetworkPolicy {
	if !scope.TokenXConfig.Enabled {
		return nil
	}

	// fromNamespace is implicitly the namespace where the egress is created
	// fromApp is the application referenced in SecurityConfig
	fromApp := scope.SecurityConfig.Spec.ApplicationRef

	toNamespace := config.Get().TokenxNamespace
	toApp := config.Get().TokenxName

	egressRules := []v1.NetworkPolicyEgressRule{
		{
			To: []v1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": toNamespace,
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": toApp,
						},
					},
				},
			},
		},
	}

	// OPA discovery server is reconciled in the application namespace. Allow app pods
	// to fetch discovery docs when both TokenX egress policy and OPA are enabled.
	if scope.OpaConfig.Enabled {
		egressRules = append(egressRules, v1.NetworkPolicyEgressRule{
			To: []v1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name":      utilities.GetOpaDiscoveryServiceName(scope.SecurityConfig.Spec.ApplicationRef),
							"app.kubernetes.io/component": "opa-discovery",
						},
					},
				},
			},
		})
	}

	return &v1.NetworkPolicy{
		ObjectMeta: objectMeta,
		Spec: v1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": fromApp,
				},
			},
			PolicyTypes: []v1.PolicyType{
				v1.PolicyTypeEgress,
			},
			Egress: egressRules,
		},
	}
}
