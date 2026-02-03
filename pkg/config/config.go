package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	ClusterName        string `split_words:"true"`
	TokenxName         string `split_words:"true"`
	TokenxNamespace    string `split_words:"true"`
	TexasImageName     string `split_words:"true" default:"ghcr.io/nais/texas"`
	TexasImageTag      string `split_words:"true" default:"latest"`
	TexasPort          int32  `split_words:"true" default:"3000"`
	TexasUrlEnvVarName string `split_words:"true" default:"TEXAS_URL"`
	OpaImageName       string `split_words:"true" default:"openpolicyagent/opa"`
	OpaImageTag        string `split_words:"true" default:"1.9.0-istio-5-static"`
	OpaPort            int32  `split_words:"true" default:"8181"`
	OpaUrlEnvVarName   string `split_words:"true" default:"OPA_URL"`
}

var cfg Config

func Load() error {
	return envconfig.Process("accesserator", &cfg)
}

func Get() Config {
	return cfg
}
