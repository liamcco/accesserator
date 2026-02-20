package opa

import (
	_ "embed"

	"gopkg.in/yaml.v3"

	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/utilities"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OPAConfig struct {
	Plugins      map[string]EnvoyExtAuthzGrpc `yaml:"plugins"`
	DecisionLogs DecisionLogs                 `yaml:"decision_logs"`
	Services     map[string]Service           `yaml:"services"`
	Discovery    Discovery                    `yaml:"discovery"`
	Keys         map[string]Key               `yaml:"keys"`
}

type EnvoyExtAuthzGrpc struct {
	Addr string `yaml:"addr"`
	Path string `yaml:"path"`
}

type DecisionLogs struct {
	Console bool `yaml:"console"`
}

type Service struct {
	URL         string       `yaml:"url"`
	Type        string       `yaml:"type,omitempty"`
	Credentials *Credentials `yaml:"credentials,omitempty"`
}

type Credentials struct {
	Bearer Bearer `yaml:"bearer"`
}

type Bearer struct {
	Scheme string       `yaml:"scheme"`
	Token  QuotedString `yaml:"token"`
}

type Bundle struct {
	Service  string  `yaml:"service" json:"service"`
	Resource string  `yaml:"resource" json:"resource"`
	Polling  Polling `yaml:"polling" json:"polling"`
	Signing  Signing `yaml:"signing" json:"signing"`
}

type Discovery struct {
	Service  string  `yaml:"service" json:"service"`
	Resource string  `yaml:"resource" json:"resource"`
	Polling  Polling `yaml:"polling" json:"polling"`
}

type Polling struct {
	MinDelaySeconds int `yaml:"min_delay_seconds" json:"min_delay_seconds"`
	MaxDelaySeconds int `yaml:"max_delay_seconds" json:"max_delay_seconds"`
}

type Signing struct {
	KeyID string `yaml:"keyid" json:"keyid"`
}

type Key struct {
	Algorithm string       `yaml:"algorithm"`
	Key       QuotedString `yaml:"key"`
}

type QuotedString string

func (q QuotedString) MarshalYAML() (interface{}, error) {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: string(q),
		Style: yaml.DoubleQuotedStyle,
	}, nil
}

func GetDesired(objectMeta v1.ObjectMeta, scope state.Scope) *corev1.ConfigMap {
	if !scope.OpaConfig.Enabled {
		return nil
	}

	githubTokenVar := QuotedString("${" + utilities.OpaGithubTokenEnvVar + "}")
	publicKeyVar := QuotedString("${" + utilities.OpaPublicKeyEnvVar + "}")

	cfg := OPAConfig{
		Plugins: map[string]EnvoyExtAuthzGrpc{
			"envoy_ext_authz_grpc": {
				Addr: ":9191",
				Path: "istio/authz/allow",
			},
		},
		DecisionLogs: DecisionLogs{Console: true},
		Services: map[string]Service{
			"ghcr-registry": {
				URL:  "https://ghcr.io",
				Type: "oci",
				Credentials: &Credentials{
					Bearer: Bearer{
						Scheme: "Bearer",
						Token:  githubTokenVar,
					},
				},
			},
			"discovery-server": {
				URL: "http://" + utilities.GetOpaDiscoveryServiceName(scope.SecurityConfig.Spec.ApplicationRef) + "." + scope.SecurityConfig.Namespace + ".svc.cluster.local",
			},
		},
		Discovery: Discovery{
			Service:  "discovery-server",
			Resource: GetOpaDiscoveryResourcePath(),
			Polling: Polling{
				MinDelaySeconds: 10,
				MaxDelaySeconds: 30,
			},
		},
		Keys: map[string]Key{
			"bundle-verification-key": {
				Algorithm: "RS256",
				Key:       publicKeyVar,
			},
		},
	}

	configYAML, err := yaml.Marshal(cfg)
	if err != nil {
		return nil
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: objectMeta,
		Data: map[string]string{
			utilities.OpaConfigFileName: string(configYAML),
		},
	}

	return configMap
}
