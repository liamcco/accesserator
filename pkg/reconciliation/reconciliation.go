package reconciliation

import (
	"context"
	"fmt"
	"reflect"

	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ControllerResource interface {
	Reconcile(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme) (ctrl.Result, error)
	GetResourceKind() string
	GetResourceName() string
	IsResourceNil() bool
}

type ReconcilerAdapter[T client.Object] struct {
	Func ResourceReconciler[T]
}

type ResourceReconciler[T client.Object] struct {
	ResourceKind    string
	ResourceName    string
	DesiredResource *T
	Scope           *state.Scope
	ShouldUpdate    func(current T, desired T) bool
	UpdateFields    func(current T, desired T)
}

func CountReconciledResources(rfs []ControllerResource) int {
	count := 0
	for _, rf := range rfs {
		if !rf.IsResourceNil() {
			count++
		}
	}
	return count
}

func ReconcileControllerResource[T client.Object](
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
	scope *state.Scope,
	resourceKind, resourceName string,
	desired *T,
	shouldUpdate func(current, desired T) bool,
	updateFields func(current, desired T),
) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)
	if desired == nil || reflect.ValueOf(*desired).IsNil() {
		// Resource is not desired. Try deleting the existing one if it exists.
		resourceType := reflect.TypeOf((*T)(nil)).Elem()
		current, _ := reflect.New(resourceType.Elem()).Interface().(T)

		accessor := current
		accessor.SetNamespace(scope.SecurityConfig.Namespace)
		accessor.SetName(resourceName)

		rLog.Info(
			fmt.Sprintf(
				"Desired %s %s/%s is nil. Will try to delete it if it exist",
				resourceKind,
				accessor.GetNamespace(),
				accessor.GetName(),
			),
		)
		rLog.Debug(
			fmt.Sprintf("Checking if %s %s/%s exists", resourceKind, accessor.GetNamespace(), accessor.GetName()),
		)

		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(accessor), current)
		if err != nil {
			if apierrors.IsNotFound(err) {
				rLog.Debug(
					fmt.Sprintf("%s %s/%s already deleted", resourceKind, accessor.GetNamespace(), accessor.GetName()),
				)
				return ctrl.Result{}, nil
			}
			getErrorMessage := fmt.Sprintf(
				"Failed to get %s %s/%s when trying to delete it.",
				resourceKind,
				accessor.GetNamespace(),
				accessor.GetName(),
			)
			rLog.Error(err, getErrorMessage)
			scope.ReplaceDescendant(accessor, &getErrorMessage, nil, resourceKind, resourceName)
			return ctrl.Result{}, err
		}

		rLog.Info(
			fmt.Sprintf(
				"Deleting %s %s/%s as it's no longer desired",
				resourceKind,
				accessor.GetNamespace(),
				accessor.GetName(),
			),
		)
		if deleteErr := k8sClient.Delete(ctx, current); deleteErr != nil {
			deleteErrorMessage := fmt.Sprintf(
				"Failed to delete %s %s/%s",
				resourceKind,
				accessor.GetNamespace(),
				accessor.GetName(),
			)
			rLog.Error(deleteErr, deleteErrorMessage)
			scope.ReplaceDescendant(accessor, &deleteErrorMessage, nil, resourceKind, resourceName)
			return ctrl.Result{}, deleteErr
		}

		rLog.Debug(
			fmt.Sprintf("Successfully deleted %s %s/%s", resourceKind, accessor.GetNamespace(), accessor.GetName()),
		)
		successMsg := fmt.Sprintf(
			"Deleted %s %s/%s as it is no longer desired.",
			resourceKind,
			accessor.GetNamespace(),
			accessor.GetName(),
		)
		scope.ReplaceDescendant(accessor, nil, &successMsg, resourceKind, resourceName)
		return ctrl.Result{}, nil
	}

	deReferencedDesired := *desired

	kind := reflect.TypeOf(deReferencedDesired).Elem().Name()
	current, _ := reflect.New(reflect.TypeOf(deReferencedDesired).Elem()).Interface().(T)

	rLog.Info(
		fmt.Sprintf(
			"Trying to generate %s %s/%s",
			kind,
			deReferencedDesired.GetNamespace(),
			deReferencedDesired.GetName(),
		),
	)

	rLog.Debug(
		fmt.Sprintf(
			"Checking if %s %s/%s exists",
			kind,
			deReferencedDesired.GetNamespace(),
			deReferencedDesired.GetName(),
		),
	)
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(deReferencedDesired), current)
	if apierrors.IsNotFound(err) {
		rLog.Debug(
			fmt.Sprintf(
				"%s %s/%s does not exist",
				kind,
				deReferencedDesired.GetNamespace(),
				deReferencedDesired.GetName(),
			),
		)
		if controllerRefErr := ctrl.SetControllerReference(
			&scope.SecurityConfig,
			deReferencedDesired,
			scheme,
		); controllerRefErr != nil {
			errorReason := fmt.Sprintf(
				"Unable to set ownerReference on %s %s/%s.",
				kind,
				deReferencedDesired.GetNamespace(),
				deReferencedDesired.GetName(),
			)
			scope.ReplaceDescendant(deReferencedDesired, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, controllerRefErr
		}

		rLog.Info(
			fmt.Sprintf("Creating %s %s/%s", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName()),
		)
		if createErr := k8sClient.Create(ctx, deReferencedDesired); createErr != nil {
			errorReason := fmt.Sprintf(
				"Unable to create %s %s/%s",
				kind,
				deReferencedDesired.GetNamespace(),
				deReferencedDesired.GetName(),
			)
			scope.ReplaceDescendant(deReferencedDesired, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, createErr
		}
		successMessage := fmt.Sprintf(
			"Successfully created %s %s/%s.",
			kind,
			deReferencedDesired.GetNamespace(),
			deReferencedDesired.GetName(),
		)
		scope.ReplaceDescendant(deReferencedDesired, nil, &successMessage, resourceKind, resourceName)

		return ctrl.Result{}, nil
	}

	if err != nil {
		errorReason := fmt.Sprintf(
			"Unable to get %s %s/%s.",
			kind,
			deReferencedDesired.GetNamespace(),
			deReferencedDesired.GetName(),
		)
		scope.ReplaceDescendant(deReferencedDesired, &errorReason, nil, resourceKind, resourceName)
		return ctrl.Result{}, err
	}

	rLog.Debug(fmt.Sprintf("%s %s/%s exists", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName()))
	rLog.Debug(
		fmt.Sprintf(
			"Determine if %s %s/%s should be updated",
			kind,
			deReferencedDesired.GetNamespace(),
			deReferencedDesired.GetName(),
		),
	)
	if shouldUpdate(current, deReferencedDesired) {
		rLog.Debug(
			fmt.Sprintf(
				"Current %s %s/%s != desired",
				kind,
				deReferencedDesired.GetNamespace(),
				deReferencedDesired.GetName(),
			),
		)
		rLog.Debug(
			fmt.Sprintf(
				"Updating current %s %s/%s with desired",
				kind,
				deReferencedDesired.GetNamespace(),
				deReferencedDesired.GetName(),
			),
		)
		before := current.DeepCopyObject().(client.Object)
		updateFields(current, deReferencedDesired)

		if patchErr := k8sClient.Patch(ctx, current, client.MergeFrom(before)); patchErr != nil {
			errorReason := fmt.Sprintf(
				"Unable to patch %s %s/%s.",
				kind,
				current.GetNamespace(),
				current.GetName(),
			)
			scope.ReplaceDescendant(current, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, patchErr
		}
	} else {
		rLog.Debug(
			fmt.Sprintf(
				"Current %s %s/%s == desired. No update needed.",
				kind,
				deReferencedDesired.GetNamespace(),
				deReferencedDesired.GetName(),
			),
		)
	}

	successMessage := fmt.Sprintf(
		"Successfully generated %s %s/%s",
		kind,
		current.GetNamespace(),
		current.GetName(),
	)
	rLog.Info(successMessage)
	scope.ReplaceDescendant(current, nil, &successMessage, resourceKind, resourceName)

	return ctrl.Result{}, nil
}
