package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	ClusterName     string `split_words:"true"`
	TokenxName      string `split_words:"true"`
	TokenxNamespace string `split_words:"true"`
}

var cfg Config

func Load() error {
	return envconfig.Process("accesserator", &cfg)
}

func Get() Config {
	return cfg
}
