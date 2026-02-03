package opa

import (
	_ "embed"

	"github.com/kartverket/accesserator/internal/state"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed opa_config.yaml
var opaConfigYAML string

func GetDesired(objectMeta v1.ObjectMeta, scope state.Scope) *corev1.ConfigMap {
	if !scope.OpaConfig.Enabled {
		return nil
	}
	return &corev1.ConfigMap{
		ObjectMeta: objectMeta,
		Data: map[string]string{
			"config.yaml": opaConfigYAML,
		},
	}
}
