package jwker

import (
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(objectMeta v1.ObjectMeta, scope state.Scope) *nais_io_v1.Jwker {
	if !scope.TokenXConfig.Enabled {
		return nil
	}
	return &nais_io_v1.Jwker{
		ObjectMeta: objectMeta,
		Spec: nais_io_v1.JwkerSpec{
			SecretName:   utilities.GetJwkerSecretName(objectMeta.Name),
			AccessPolicy: getNaisIoV1AccessPolicy(scope.TokenXConfig.AccessPolicy, scope.SecurityConfig.Namespace),
		},
	}
}

func getNaisIoV1AccessPolicy(skiperatorAccessPolicy *podtypes.AccessPolicy, securityConfigNamespace string) *nais_io_v1.AccessPolicy {
	if skiperatorAccessPolicy == nil {
		return nil
	}

	naisIoV1AccessPolicyInboundRules := nais_io_v1.AccessPolicyInboundRules{}
	naisIoV1AccessPolicyOutboundRules := nais_io_v1.AccessPolicyRules{}
	if skiperatorAccessPolicy.Inbound != nil {
		for _, rule := range skiperatorAccessPolicy.Inbound.Rules {
			naisIoV1AccessPolicyInboundRules = append(
				naisIoV1AccessPolicyInboundRules,
				nais_io_v1.AccessPolicyInboundRule{
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

	return &nais_io_v1.AccessPolicy{
		Inbound: &nais_io_v1.AccessPolicyInbound{
			Rules: naisIoV1AccessPolicyInboundRules,
		},
		Outbound: &nais_io_v1.AccessPolicyOutbound{
			Rules: naisIoV1AccessPolicyOutboundRules,
		},
	}
}

func getNaisIoV1AccessPolicyRule(skiperatorAccessPolicyRule podtypes.InternalRule, securityConfigNamespace string) nais_io_v1.AccessPolicyRule {
	var accessPolicyNamespace string
	if skiperatorAccessPolicyRule.Namespace != "" {
		accessPolicyNamespace = skiperatorAccessPolicyRule.Namespace
	} else {
		accessPolicyNamespace = securityConfigNamespace
	}
	return nais_io_v1.AccessPolicyRule{
		Application: skiperatorAccessPolicyRule.Application,
		Namespace:   accessPolicyNamespace,
		Cluster:     config.Get().ClusterName,
	}
}
