package utilities

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestPtr(t *testing.T) {
	v := 42
	ptr := Ptr(v)
	assert.NotNil(t, ptr)
	assert.Equal(t, v, *ptr)
}

func TestLowestNonZeroResult(t *testing.T) {
	zero := ctrl.Result{}
	one := ctrl.Result{RequeueAfter: 1 * time.Second}
	two := ctrl.Result{RequeueAfter: 2 * time.Second}

	assert.Equal(t, zero, LowestNonZeroResult(zero, zero))
	assert.Equal(t, one, LowestNonZeroResult(zero, one))
	assert.Equal(t, one, LowestNonZeroResult(one, zero))
	assert.Equal(t, one, LowestNonZeroResult(one, two))
	assert.Equal(t, one, LowestNonZeroResult(two, one))
}

func TestGetJwkerName(t *testing.T) {
	appRef := "my-app"
	assert.Equal(t, appRef, GetJwkerName(appRef))
}

func TestGetJwkerSecretName(t *testing.T) {
	jwkerName := "foo"
	want := fmt.Sprintf("%s-%s", jwkerName, JwkerSecretNameSuffix)
	assert.Equal(t, want, GetJwkerSecretName(jwkerName))
}

func TestGetOpaDiscoveryNames(t *testing.T) {
	appRef := "my-app"
	assert.Equal(
		t,
		fmt.Sprintf("%s-%s", appRef, OpaDiscoveryConfigNameSuffix),
		GetOpaDiscoveryConfigName(appRef),
	)
	assert.Equal(
		t,
		fmt.Sprintf("%s-%s", appRef, OpaDiscoveryServiceNameSuffix),
		GetOpaDiscoveryServiceName(appRef),
	)
	assert.Equal(
		t,
		fmt.Sprintf("%s-%s", appRef, OpaDiscoveryDeploymentNameSuffix),
		GetOpaDiscoveryDeploymentName(appRef),
	)
	assert.Equal(
		t,
		fmt.Sprintf("%s-%s", appRef, OpaDiscoveryEgressNameSuffix),
		GetOpaDiscoveryEgressPolicyName(appRef),
	)
	assert.Equal(
		t,
		fmt.Sprintf("%s-%s", appRef, OpaDiscoveryIngressNameSuffix),
		GetOpaDiscoveryIngressPolicyName(appRef),
	)
}

func TestGetTokenxEgressName(t *testing.T) {
	secName := "sec"
	tokenx := "tok"
	want := fmt.Sprintf("%s-%s-%s", secName, tokenx, EgressNameSuffix)
	assert.Equal(t, want, GetTokenxEgressName(secName, tokenx))
}

func TestGetMockKubernetesClient(t *testing.T) {
	scheme := runtime.NewScheme()
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test-cm")
	client := GetMockKubernetesClient(scheme, obj)
	assert.NotNil(t, client)
}
