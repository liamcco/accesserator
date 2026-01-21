package utilities

import (
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

func Ptr[T any](v T) *T {
	return &v
}

func LowestNonZeroResult(i, j ctrl.Result) ctrl.Result {
	switch {
	case i.IsZero() && j.IsZero():
		return ctrl.Result{}
	case i.IsZero():
		return j
	case j.IsZero():
		return i
	case i.RequeueAfter != 0 && j.RequeueAfter != 0:
		if i.RequeueAfter < j.RequeueAfter {
			return i
		}
		return j
	case i.RequeueAfter != 0:
		return i
	case j.RequeueAfter != 0:
		return j
	default:
		return ctrl.Result{RequeueAfter: 0 * time.Second}
	}
}

func GetJwkerName(applicationRef string) string {
	return applicationRef
}

func GetJwkerSecretName(jwkerName string) string {
	return fmt.Sprintf("%s-%s", jwkerName, JwkerSecretNameSuffix)
}

func GetTokenxEgressName(securityConfigName string, tokenxConfigName string) string {
	return fmt.Sprintf("%s-%s-%s", securityConfigName, tokenxConfigName, EgressNameSuffix)
}
