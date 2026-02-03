package state

import (
	"context"
	"fmt"
	"reflect"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Scope struct {
	SecurityConfig         v1alpha.SecurityConfig
	TokenXConfig           TokenXConfig
	OpaConfig              OpaConfig
	Descendants            []Descendant[client.Object]
	InvalidConfig          bool
	ValidationErrorMessage *string
}

type TokenXConfig struct {
	Enabled      bool
	AccessPolicy *podtypes.AccessPolicy
}

type OpaConfig struct {
	Enabled bool
}

type Descendant[T client.Object] struct {
	ID             string
	Object         T
	ErrorMessage   *string
	SuccessMessage *string
}

func (s *Scope) GetErrors() []string {
	var errs []string
	if s != nil {
		for _, d := range s.Descendants {
			if d.ErrorMessage != nil {
				errs = append(errs, *d.ErrorMessage)
			}
		}
	}
	return errs
}

func (s *Scope) ReplaceDescendant(
	obj client.Object,
	errorMessage *string,
	successMessage *string,
	resourceKind, resourceName string,
) {
	if s != nil {
		for i, d := range s.Descendants {
			if reflect.TypeOf(d) == reflect.TypeOf(obj) && d.ID == obj.GetName() {
				s.Descendants[i] = Descendant[client.Object]{
					Object:         obj,
					ErrorMessage:   errorMessage,
					SuccessMessage: successMessage,
				}
				return
			}
		}
		s.Descendants = append(s.Descendants, Descendant[client.Object]{
			ID:             GetID(resourceKind, resourceName),
			Object:         obj,
			ErrorMessage:   errorMessage,
			SuccessMessage: successMessage,
		})
	}
}

func GetID(resourceKind, resourceName string) string {
	return fmt.Sprintf("%s-%s", resourceKind, resourceName)
}

func (s *Scope) IsMisconfigured() bool {
	return s.InvalidConfig
}

func (s *Scope) GetJwker(ctx context.Context, k8sClient client.Client) (*nais_io_v1.Jwker, error) {
	var jwker nais_io_v1.Jwker
	if k8sClient == nil {
		return nil, fmt.Errorf("k8sClient not configured")
	}
	jwkerName := utilities.GetJwkerName(s.SecurityConfig.Spec.ApplicationRef)
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      jwkerName,
		Namespace: s.SecurityConfig.Namespace,
	}, &jwker); err != nil {
		return nil, fmt.Errorf("failed to fetch Jwker resource named %s: %w", jwkerName, err)
	}
	return &jwker, nil
}

func (s *Scope) GetOpaConfig(ctx context.Context, k8sClient client.Client) (*corev1.ConfigMap, error) {
	var configMap corev1.ConfigMap
	if k8sClient == nil {
		return nil, fmt.Errorf("k8sClient not configured")
	}
	opaConfigName := utilities.GetOpaConfigName(s.SecurityConfig.Spec.ApplicationRef)
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      opaConfigName,
		Namespace: s.SecurityConfig.Namespace,
	}, &configMap); err != nil {
		return nil, fmt.Errorf("failed to fetch Opa config resource named %s: %w", opaConfigName, err)
	}
	return &configMap, nil
}
