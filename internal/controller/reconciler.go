package controller

import (
	"context"
	"reflect"

	"github.com/kartverket/accesserator/pkg/reconciliation"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ControllerResourceAdapter[T client.Object] struct {
	reconciliation.ReconcilerAdapter[T]
}

func (c ControllerResourceAdapter[T]) Reconcile(
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
) (ctrl.Result, error) {
	return reconciliation.ReconcileControllerResource(
		ctx,
		k8sClient,
		scheme,
		c.Func.Scope,
		c.Func.ResourceKind,
		c.Func.ResourceName,
		c.Func.DesiredResource,
		c.Func.ShouldUpdate,
		c.Func.UpdateFields,
	)
}

func (c ControllerResourceAdapter[T]) GetResourceKind() string {
	return c.Func.ResourceKind
}

func (c ControllerResourceAdapter[T]) GetResourceName() string {
	return c.Func.ResourceName
}

func (c ControllerResourceAdapter[T]) IsResourceNil() bool {
	return c.Func.DesiredResource == nil || reflect.ValueOf(*c.Func.DesiredResource).IsNil()
}
