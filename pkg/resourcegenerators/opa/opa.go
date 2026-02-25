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
	Plugins      map[string]EnvoyExtAuthzGrpc `yaml:"plugins,omitempty"`
	DecisionLogs DecisionLogs                 `yaml:"decision_logs,omitempty"`
	Services     map[string]Service           `yaml:"services,omitempty"`
	Bundles      map[string]Bundle            `yaml:"bundles,omitempty"`
	Keys         map[string]Key               `yaml:"keys,omitempty"`
}

type EnvoyExtAuthzGrpc struct {
	Addr string `yaml:"addr"`
	Path string `yaml:"path"`
}

type DecisionLogs struct {
	Console bool `yaml:"console"`
}

type Service struct {
	URL         string      `yaml:"url"`
	Type        string      `yaml:"type"`
	Credentials Credentials `yaml:"credentials"`
}

type Credentials struct {
	Bearer Bearer `yaml:"bearer"`
}

type Bearer struct {
	Scheme string       `yaml:"scheme"`
	Token  QuotedString `yaml:"token"`
}

type Bundle struct {
	Service  string  `yaml:"service"`
	Resource string  `yaml:"resource"`
	Polling  Polling `yaml:"polling"`
	Signing  Signing `yaml:"signing"`
}

type Polling struct {
	MinDelaySeconds int `yaml:"min_delay_seconds"`
	MaxDelaySeconds int `yaml:"max_delay_seconds"`
}

type Signing struct {
	KeyID string `yaml:"keyid"`
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

	cfg := OPAConfig{
		Plugins: map[string]EnvoyExtAuthzGrpc{
			"envoy_ext_authz_grpc": {
				Addr: ":9191",
				Path: "istio/authz/allow",
			},
		},
		DecisionLogs: DecisionLogs{Console: true},
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

func GetBundleDesired(objectMeta v1.ObjectMeta, scope state.Scope, bundle []byte) *corev1.ConfigMap {
	if !scope.OpaConfig.Enabled {
		return nil
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: objectMeta,
		BinaryData: map[string][]byte{
			utilities.OpaBundleFileName: bundle,
		},
	}

	return configMap
}
