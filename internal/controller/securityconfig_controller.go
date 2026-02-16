package controller

import (
	"context"
	"fmt"
	"strings"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/eventhandler"
	"github.com/kartverket/accesserator/internal/resolver"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/accesserator/pkg/reconciliation"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/tokenx/egress"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/tokenx/jwker"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/skiperator/api/v1alpha1"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	networkv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sErrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const jwkerSynchronizationStateReady = "RolloutComplete"

// SecurityConfigReconciler reconciles a SecurityConfig object
type SecurityConfigReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecurityConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&accesseratorv1alpha.SecurityConfig{}).
		Owns(&naisiov1.Jwker{}).
		Owns(&networkv1.NetworkPolicy{}).
		Watches(&v1alpha1.Application{}, eventhandler.HandleSkiperatorApplicationEvent(r.Client)).
		Named("securityconfig").
		Complete(r)
}

// +kubebuilder:rbac:groups=accesserator.kartverket.no,resources=securityconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=accesserator.kartverket.no,resources=securityconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=accesserator.kartverket.no,resources=securityconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=skiperator.kartverket.no,resources=applications,verbs=get;list;watch
// +kubebuilder:rbac:groups=nais.io,resources=jwkers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete

func (r *SecurityConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rlog := log.GetLogger(ctx)
	securityConfig := new(accesseratorv1alpha.SecurityConfig)
	rlog.Info("Reconciling SecurityConfig", "name", req.NamespacedName)

	if err := r.Get(ctx, req.NamespacedName, securityConfig); err != nil {
		if apierrors.IsNotFound(err) {
			rlog.Debug("SecurityConig with not found. Probably a delete.", "name", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		rlog.Error(err, "failed to get SecurityConfig", "name", req.NamespacedName)
		return reconcile.Result{}, err
	}

	r.Recorder.Eventf(
		securityConfig,
		"Normal",
		"ReconcileStarted",
		fmt.Sprintf("SecurityConfig with name %s started.", req.String()),
	)
	rlog.Debug("SecurityConfig found", "name", req.NamespacedName)

	securityConfig.InitializeStatus()
	deepCopiedSecurityConfig := securityConfig.DeepCopy()

	if !securityConfig.DeletionTimestamp.IsZero() {
		rlog.Info("SecurityConfig is marked for deletion.", "name", req.NamespacedName)
		return reconcile.Result{}, nil
	}

	scope, err := resolver.ResolveSecurityConfig(ctx, r.Client, *securityConfig)
	if err != nil {
		rlog.Error(err, "failed to resolve SecurityConfig", "name", req.NamespacedName)
		securityConfig.Status.Phase = accesseratorv1alpha.PhaseFailed
		securityConfig.Status.Message = err.Error()
		updateStatusOnResolveFailedErr := r.updateStatusWithRetriesOnConflict(ctx, *securityConfig)
		if updateStatusOnResolveFailedErr != nil {
			return ctrl.Result{}, updateStatusOnResolveFailedErr
		}
		return reconcile.Result{}, err
	}

	jwkerObjectMeta := metav1.ObjectMeta{
		Name:      utilities.GetJwkerName(securityConfig.Spec.ApplicationRef),
		Namespace: securityConfig.Namespace,
	}

	tokenxEgressObjectMeta := metav1.ObjectMeta{
		Name:      utilities.GetTokenxEgressName(securityConfig.Name, config.Get().TokenxName),
		Namespace: securityConfig.Namespace,
	}

	controllerResources := []reconciliation.ControllerResource{
		ControllerResourceAdapter[*naisiov1.Jwker]{
			reconciliation.ReconcilerAdapter[*naisiov1.Jwker]{
				Func: reconciliation.ResourceReconciler[*naisiov1.Jwker]{
					ResourceKind:    "Jwker",
					ResourceName:    jwkerObjectMeta.Name,
					DesiredResource: utilities.Ptr(jwker.GetDesired(jwkerObjectMeta, *scope)),
					Scope:           scope,
					ShouldUpdate: func(current, desired *naisiov1.Jwker) bool {
						return !equality.Semantic.DeepEqual(current.Spec, desired.Spec)
					},
					UpdateFields: func(current, desired *naisiov1.Jwker) {
						current.Spec = desired.Spec
					},
				},
			},
		},
		ControllerResourceAdapter[*networkv1.NetworkPolicy]{
			reconciliation.ReconcilerAdapter[*networkv1.NetworkPolicy]{
				Func: reconciliation.ResourceReconciler[*networkv1.NetworkPolicy]{
					ResourceKind:    "NetworkPolicy",
					ResourceName:    tokenxEgressObjectMeta.Name,
					DesiredResource: utilities.Ptr(egress.GetDesired(tokenxEgressObjectMeta, *scope)),
					Scope:           scope,
					ShouldUpdate: func(current, desired *networkv1.NetworkPolicy) bool {
						return !equality.Semantic.DeepEqual(current.Spec, desired.Spec)
					},
					UpdateFields: func(current, desired *networkv1.NetworkPolicy) {
						current.Spec = desired.Spec
					},
				},
			},
		},
	}

	defer func() {
		r.updateStatus(ctx, scope, deepCopiedSecurityConfig, controllerResources)
	}()

	return r.doReconcile(ctx, controllerResources, scope)
}

func (r *SecurityConfigReconciler) doReconcile(
	ctx context.Context,
	controllerResources []reconciliation.ControllerResource,
	scope *state.Scope,
) (ctrl.Result, error) {
	result := ctrl.Result{}
	var errs []error
	for _, rf := range controllerResources {
		reconcileResult, err := rf.Reconcile(ctx, r.Client, r.Scheme)
		if err != nil {
			r.Recorder.Eventf(
				&scope.SecurityConfig,
				"Warning",
				fmt.Sprintf("%sReconcileFailed", rf.GetResourceKind()),
				fmt.Sprintf(
					"%s with name %s failed during reconciliation.",
					rf.GetResourceKind(),
					rf.GetResourceName(),
				),
			)
			errs = append(errs, err)
		} else {
			r.Recorder.Eventf(&scope.SecurityConfig, "Normal", fmt.Sprintf("%sReconciledSuccessfully", rf.GetResourceKind()), fmt.Sprintf("%s with name %s reconciled successfully.", rf.GetResourceKind(), rf.GetResourceName()))
		}
		if len(errs) > 0 {
			continue
		}
		result = utilities.LowestNonZeroResult(result, reconcileResult)
	}

	if len(errs) > 0 {
		r.Recorder.Eventf(&scope.SecurityConfig, "Error", "ReconcileFailed", "SecurityConfig failed during reconciliation")
		r.Recorder.Eventf(&scope.SecurityConfig, "Error", "ReconcileFailed", "SecurityConfig failed during reconciliation")
		return ctrl.Result{}, k8sErrors.NewAggregate(errs)
	}
	r.Recorder.Eventf(&scope.SecurityConfig, "Normal", "ReconcileSuccess", "SecurityConfig reconciled successfully")
	return result, nil
}

func (r *SecurityConfigReconciler) updateStatusWithRetriesOnConflict(
	ctx context.Context,
	securityConfig accesseratorv1alpha.SecurityConfig,
) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &accesseratorv1alpha.SecurityConfig{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(&securityConfig), latest); err != nil {
			return err
		}
		latest.Status = securityConfig.Status
		return r.Status().Update(ctx, latest)
	})
}

func (r *SecurityConfigReconciler) updateStatus(
	ctx context.Context,
	scope *state.Scope,
	original *accesseratorv1alpha.SecurityConfig,
	controllerResources []reconciliation.ControllerResource,
) {
	securityConfig := scope.SecurityConfig
	rLog := log.GetLogger(ctx)
	rLog.Debug(fmt.Sprintf("Updating SecurityConfig status for %s/%s", securityConfig.Namespace, securityConfig.Name))

	securityConfig.Status.ObservedGeneration = securityConfig.GetGeneration()
	statusCondition := metav1.Condition{
		Type:               state.GetID(strings.TrimPrefix(securityConfig.Kind, "*"), securityConfig.Name),
		LastTransitionTime: metav1.Now(),
	}

	switch {
	case scope.InvalidConfig:
		securityConfig.Status.SetPhaseInvalid(*scope.ValidationErrorMessage)
		accesseratorv1alpha.SetConditionInvalid(&statusCondition, *scope.ValidationErrorMessage)

	case len(scope.Descendants) != reconciliation.CountReconciledResources(controllerResources):
		securityConfig.Status.SetPhasePending("SecurityConfig pending due to missing Descendants.")
		accesseratorv1alpha.SetConditionPending(&statusCondition, "Descendants of SecurityConfig are not reconciled yet.")

	case len(scope.GetErrors()) > 0:
		securityConfig.Status.SetPhaseFailed("SecurityConfig reconciliation failed.")
		accesseratorv1alpha.SetConditionFailed(&statusCondition, "Descendants of SecurityConfig failed during reconciliation.")

	case scope.TokenXConfig.Enabled:
		jwkerResource, getJwkerErr := scope.GetJwker(ctx, r.Client)
		if getJwkerErr != nil {
			rLog.Error(
				getJwkerErr,
				fmt.Sprintf(
					"Failed to get Jwker resource with name %s when updating SecurityConfig status",
					utilities.GetJwkerName(securityConfig.Spec.ApplicationRef),
				),
			)
			r.Recorder.Eventf(&securityConfig, "Error", "StatusUpdateFailed", "Failed to get Jwker resource with name %s.", utilities.GetJwkerName(securityConfig.Spec.ApplicationRef))
		}
		if jwkerResource.Status.SynchronizationState != jwkerSynchronizationStateReady {
			securityConfig.Status.SetPhasePending("SecurityConfig pending due to missing TokenX secret.")
			statusMsg := fmt.Sprintf("Jwker resource with name %s has not finished registering an OAuth client", jwkerResource.Name)
			accesseratorv1alpha.SetConditionPending(&statusCondition, statusMsg)
		} else {
			securityConfig.Status.SetPhaseReady("SecurityConfig ready.")
			accesseratorv1alpha.SetConditionReady(&statusCondition, "Descendants of SecurityConfig reconciled successfully.")
		}

	default:
		securityConfig.Status.SetPhaseReady("SecurityConfig ready.")
		accesseratorv1alpha.SetConditionReady(&statusCondition, "Descendants of SecurityConfig reconciled successfully.")
	}

	conditions := make([]metav1.Condition, 0, len(scope.Descendants)+len(controllerResources))
	descendantIDs := map[string]bool{}

	for _, d := range scope.Descendants {
		descendantIDs[d.ID] = true
		cond := metav1.Condition{
			Type:               d.ID,
			LastTransitionTime: metav1.Now(),
		}
		switch {
		case d.ErrorMessage != nil:
			cond.Status = metav1.ConditionFalse
			cond.Reason = "Error"
			cond.Message = *d.ErrorMessage
		case d.SuccessMessage != nil:
			cond.Status = metav1.ConditionTrue
			cond.Reason = "Success"
			cond.Message = *d.SuccessMessage
		default:
			cond.Status = metav1.ConditionUnknown
			cond.Reason = "Unknown"
			cond.Message = "No status message set"
		}
		conditions = append(conditions, cond)
	}
	for _, rf := range controllerResources {
		if !rf.IsResourceNil() {
			expectedID := state.GetID(rf.GetResourceKind(), rf.GetResourceName())
			if !descendantIDs[expectedID] {
				conditions = append(conditions, metav1.Condition{
					Type:   expectedID,
					Status: metav1.ConditionFalse,
					Reason: "NotFound",
					Message: fmt.Sprintf(
						"Expected resource %s of kind %s was not created",
						rf.GetResourceName(),
						rf.GetResourceKind(),
					),
					LastTransitionTime: metav1.Now(),
				})
			}
		}
	}

	securityConfig.Status.Conditions = append([]metav1.Condition{statusCondition}, conditions...)

	if !equality.Semantic.DeepEqual(original.Status, securityConfig.Status) {
		rLog.Debug(fmt.Sprintf("Updating SecurityConfig status with name %s/%s", securityConfig.Namespace, securityConfig.Name))
		if updateStatusWithRetriesErr := r.updateStatusWithRetriesOnConflict(ctx, securityConfig); updateStatusWithRetriesErr != nil {
			rLog.Error(
				updateStatusWithRetriesErr,
				fmt.Sprintf(
					"Failed to update SecurityConfig status with name %s/%s",
					securityConfig.Namespace,
					securityConfig.Name,
				),
			)
			r.Recorder.Eventf(&securityConfig, "Error", "StatusUpdateFailed", "Status update of SecurityConfig failed.")
		} else {
			r.Recorder.Eventf(&securityConfig, "Normal", "StatusUpdateSuccess", "Status of SecurityConfig updated successfully.")
		}
	}
}
