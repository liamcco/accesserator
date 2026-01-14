package state

import (
	"fmt"
	"reflect"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Scope struct {
	SecurityConfig         v1alpha.SecurityConfig
	TokenXConfig           TokenXConfig
	Descendants            []Descendant[client.Object]
	InvalidConfig          bool
	ValidationErrorMessage *string
}

type TokenXConfig struct {
	Enabled      bool
	AccessPolicy *podtypes.AccessPolicy
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
