package config

import (
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	ClusterName        string `split_words:"true"`
	TokenxName         string `split_words:"true" default:"tokendings"`
	TokenxNamespace    string `split_words:"true"`
	TexasImageName     string `split_words:"true" default:"ghcr.io/nais/texas"`
	TexasImageTag      string `split_words:"true"`
	TexasPort          int32  `split_words:"true" default:"3000"`
	TexasUrlEnvVarName string `split_words:"true" default:"TEXAS_URL"`
	OpaImageName       string `split_words:"true" default:"openpolicyagent/opa"`
	OpaImageTag        string `split_words:"true" default:"1.9.0-istio-5-static"`
	OpaPort            int32  `split_words:"true" default:"8181"`
	OpaUrlEnvVarName   string `split_words:"true" default:"OPA_URL"`
}

var cfg Config

func Load() error {
	if err := envconfig.Process("accesserator", &cfg); err != nil {
		return err
	}

	missing := make([]string, 0, 3)
	if cfg.ClusterName == "" {
		missing = append(missing, "ACCESSERATOR_CLUSTER_NAME")
	}
	if cfg.TokenxNamespace == "" {
		missing = append(missing, "ACCESSERATOR_TOKENX_NAMESPACE")
	}
	if cfg.TexasImageTag == "" {
		missing = append(missing, "ACCESSERATOR_TEXAS_IMAGE_TAG")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	return nil
}

func Get() Config {
	return cfg
}
