package opa

import (
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/utilities"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GetDiscoveryEgressNetworkPolicyDesired(
	objectMeta metav1.ObjectMeta,
	scope state.Scope,
) *networkingv1.NetworkPolicy {
	if !scope.OpaConfig.Enabled {
		return nil
	}

	fromApp := scope.SecurityConfig.Spec.ApplicationRef
	discoveryLabels := map[string]string{
		"app.kubernetes.io/name":      utilities.GetOpaDiscoveryServiceName(scope.SecurityConfig.Spec.ApplicationRef),
		"app.kubernetes.io/component": opaDiscoveryContainerName,
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: objectMeta,
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": fromApp,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: discoveryLabels,
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: utilities.Ptr(corev1.ProtocolTCP),
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: opaDiscoveryContainerPort},
						},
					},
				},
			},
		},
	}
}

func GetDiscoveryIngressNetworkPolicyDesired(
	objectMeta metav1.ObjectMeta,
	scope state.Scope,
) *networkingv1.NetworkPolicy {
	if !scope.OpaConfig.Enabled {
		return nil
	}

	fromApp := scope.SecurityConfig.Spec.ApplicationRef
	discoveryLabels := map[string]string{
		"app.kubernetes.io/name":      utilities.GetOpaDiscoveryServiceName(scope.SecurityConfig.Spec.ApplicationRef),
		"app.kubernetes.io/component": opaDiscoveryContainerName,
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: objectMeta,
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: discoveryLabels,
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": fromApp,
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: utilities.Ptr(corev1.ProtocolTCP),
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: opaDiscoveryContainerPort},
						},
					},
				},
			},
		},
	}
}
