package jwker

import (
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(objectMeta v1.ObjectMeta, scope state.Scope) *naisiov1.Jwker {
	if !scope.TokenXConfig.Enabled {
		return nil
	}
	return &naisiov1.Jwker{
		ObjectMeta: objectMeta,
		Spec: naisiov1.JwkerSpec{
			SecretName:   utilities.GetJwkerSecretName(objectMeta.Name),
			AccessPolicy: getNaisIoV1AccessPolicy(scope.TokenXConfig.AccessPolicy, scope.SecurityConfig.Namespace),
		},
	}
}

func getNaisIoV1AccessPolicy(
	skiperatorAccessPolicy *podtypes.AccessPolicy,
	securityConfigNamespace string,
) *naisiov1.AccessPolicy {
	if skiperatorAccessPolicy == nil {
		return nil
	}

	naisIoV1AccessPolicyInboundRules := naisiov1.AccessPolicyInboundRules{}
	naisIoV1AccessPolicyOutboundRules := naisiov1.AccessPolicyRules{}
	if skiperatorAccessPolicy.Inbound != nil {
		for _, rule := range skiperatorAccessPolicy.Inbound.Rules {
			naisIoV1AccessPolicyInboundRules = append(
				naisIoV1AccessPolicyInboundRules,
				naisiov1.AccessPolicyInboundRule{
					AccessPolicyRule: getNaisIoV1AccessPolicyRule(rule, securityConfigNamespace),
				},
			)
		}
	}
	for _, rule := range skiperatorAccessPolicy.Outbound.Rules {
		naisIoV1AccessPolicyOutboundRules = append(
			naisIoV1AccessPolicyOutboundRules,
			getNaisIoV1AccessPolicyRule(rule, securityConfigNamespace),
		)
	}

	return &naisiov1.AccessPolicy{
		Inbound: &naisiov1.AccessPolicyInbound{
			Rules: naisIoV1AccessPolicyInboundRules,
		},
		Outbound: &naisiov1.AccessPolicyOutbound{
			Rules: naisIoV1AccessPolicyOutboundRules,
		},
	}
}

func getNaisIoV1AccessPolicyRule(
	skiperatorAccessPolicyRule podtypes.InternalRule,
	securityConfigNamespace string,
) naisiov1.AccessPolicyRule {
	var accessPolicyNamespace string
	if skiperatorAccessPolicyRule.Namespace != "" {
		accessPolicyNamespace = skiperatorAccessPolicyRule.Namespace
	} else {
		accessPolicyNamespace = securityConfigNamespace
	}
	return naisiov1.AccessPolicyRule{
		Application: skiperatorAccessPolicyRule.Application,
		Namespace:   accessPolicyNamespace,
		Cluster:     config.Get().ClusterName,
	}
}
