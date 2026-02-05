package opa

import (
	_ "embed"

	"gopkg.in/yaml.v3"

	"github.com/kartverket/accesserator/internal/state"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OPAConfig struct {
	Plugins      map[string]EnvoyExtAuthzGrpc `yaml:"plugins"`
	DecisionLogs DecisionLogs                 `yaml:"decision_logs"`
	Services     map[string]Service           `yaml:"services"`
	Bundles      map[string]Bundle            `yaml:"bundles"`
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
	URL         string      `yaml:"url"`
	Type        string      `yaml:"type"`
	Credentials Credentials `yaml:"credentials"`
}

type Credentials struct {
	Bearer Bearer `yaml:"bearer"`
}

type Bearer struct {
	Scheme string `yaml:"scheme"`
	Token  string `yaml:"token"`
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
	Algorithm string `yaml:"algorithm"`
	Key       string `yaml:"key"`
}

func GetDesired(objectMeta v1.ObjectMeta, scope state.Scope) *corev1.ConfigMap {
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
				Credentials: Credentials{
					Bearer: Bearer{
						Scheme: "Bearer",
						Token:  "${GITHUB_TOKEN}",
					},
				},
			},
		},
		Bundles: map[string]Bundle{
			"authz": {
				Service:  "ghcr-registry",
				Resource: scope.OpaConfig.BundleUrl,
				Polling: Polling{
					MinDelaySeconds: 10,
					MaxDelaySeconds: 30,
				},
				Signing: Signing{
					KeyID: "bundle-verification-key",
				},
			},
		},
		Keys: map[string]Key{
			"bundle-verification-key": {
				Algorithm: "RS256",
				Key:       "${OPA_PUBLIC_KEY}",
			},
		},
	}

	y, _ := yaml.Marshal(cfg)
	// if err != nil {
	//	return nil
	// }

	cm := &corev1.ConfigMap{
		ObjectMeta: objectMeta,
		Data: map[string]string{
			"config.yaml": string(y),
		},
	}

	return cm
}
