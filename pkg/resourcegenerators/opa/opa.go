package opa

import (
	_ "embed"

	"gopkg.in/yaml.v3"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/utilities"
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
	Scheme string       `yaml:"scheme"`
	Token  QuotedString `yaml:"token"`
}

type GhcrSpec struct {
	BundlePath 		string 		`yaml:"url"`
	Type 			string 		`yaml:"type"`
	BundleResource 	string		`yaml:"resource"`
	Credentials 	Credentials	`yaml:"credentials"`
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

func applyGhcrBundle(cfg *OPAConfig, ghcr *v1alpha.GhcrSpec, githubTokenVar QuotedString) {
	cfg.Services = map[string]Service{
		"ghcr-registry": {
			URL:  "https://ghcr.io",
			Type: "oci",
			Credentials: Credentials{
				Bearer: Bearer{ Scheme: "Bearer", Token: githubTokenVar },
			},
		},
	}
	
	cfg.Bundles = map[string]Bundle{
		"authz": {
			Service:  "ghcr-registry",
			Resource: ghcr.BundlePath + ":" + ghcr.BundleVersion,
			Polling:  Polling{MinDelaySeconds: 10, MaxDelaySeconds: 30},
			Signing:  Signing{KeyID: "bundle-verification-key"},
		},
	}
}

func applyPvBundle(cfg *OPAConfig, pv *v1alpha.PvSpec) {
  	cfg.Services = map[string]Service{
		"local-bundle": {
			URL:  pv.BundlePath,
		},
	}

  	cfg.Bundles = map[string]Bundle{
    	"authz": {
			Service: "local-bundle",
			Resource: pv.BundleResource,
			Polling:  Polling{MinDelaySeconds: 10, MaxDelaySeconds: 30},
			Signing:  Signing{KeyID: "bundle-verification-key"},
    	},
  	}
}

func GetDesired(objectMeta v1.ObjectMeta, scope state.Scope) *corev1.ConfigMap {
	if !scope.OpaConfig.Enabled {
		return nil
	}

	githubTokenVar := QuotedString("${" + utilities.OpaGithubTokenEnvVar + "}")
	publicKeyVar := QuotedString("${" + utilities.OpaPublicKeyEnvVar + "}")

	cfg := OPAConfig{
		Plugins: map[string]EnvoyExtAuthzGrpc{
		"envoy_ext_authz_grpc": { Addr: ":9191", Path: "istio/authz/allow" },
		},
		DecisionLogs: DecisionLogs{Console: true},

		Keys: map[string]Key{
			"bundle-verification-key": { Algorithm: "RS256", Key: publicKeyVar },
		},
  	}

	opa := scope.SecurityConfig.Spec.Opa

	switch {
		case opa.Ghcr != nil:
			applyGhcrBundle(&cfg, opa.Ghcr, githubTokenVar)

		case opa.Pv != nil:
			applyPvBundle(&cfg, opa.Pv)

		default:
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


/**

	cfg1 := OPAConfig{
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
						Token:  githubTokenVar,
					},
				},
			},
		},
		Bundles: map[string]Bundle{
			"authz": {
				Service:  "ghcr-registry",
				Resource: scope.OpaConfig.Opa.BundlePath,
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
				Key:       publicKeyVar,
			},
		},
	}

*/