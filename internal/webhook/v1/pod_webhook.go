package v1

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/skiperator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	SkiperatorApplicationRefLabel = "application.skiperator.no/app-name"
	SecurityEnabledLabelName      = "skiperator/security"
	SecurityEnabledLabelValue     = "enabled"

	TexasInitContainerName = "texas"
	TexasPortName          = "http"

	OpaInitContainerName = "opa"
	OpaPortName          = "http"
	OpaTmpVolumeName     = "opa-tmp"
	OpaConfigMountPath   = "/config"

	MaskinportenEnabledEnvVarName = "MASKINPORTEN_ENABLED"
	AzureEnabledEnvVarName        = "AZURE_ENABLED"
	IdportenEnabledEnvVarName     = "IDPORTEN_ENABLED"
	TokenXEnabledEnvVarName       = "TOKEN_X_ENABLED"
	OpaEnabledEnvVarName          = "OPA_ENABLED"
)

// nolint:unused
// log is for logging in this package.
var podlog = logf.Log.WithName("pod-webhook")

// SetupPodWebhookWithManager registers the webhook for Pod in the manager.
func SetupPodWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&corev1.Pod{}).
		WithValidator(&PodCustomValidator{Client: mgr.GetClient()}).
		WithDefaulter(&PodCustomDefaulter{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate--v1-pod,mutating=true,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create,versions=v1,name=mpod-v1.kb.io,admissionReviewVersions=v1

// PodCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Pod when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type PodCustomDefaulter struct {
	Client client.Client
}

var _ webhook.CustomDefaulter = &PodCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Pod.
func (d *PodCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("expected an Pod object but got %T", obj)
	}

	podlog.Info("Defaulting for Pod")

	securityConfigForPod, err := getSecurityConfigForPod(ctx, d.Client, pod)
	if err != nil {
		return err
	}
	if !securityConfigForPod.SecurityEnabled {
		return nil
	}

	if securityConfigForPod.SecurityConfig.Spec.Tokenx != nil && securityConfigForPod.SecurityConfig.Spec.Tokenx.Enabled {
		// TokenX is enabled for this Application
		// We inject an init container with texas in the pod
		podlog.Info("Tokenx is enabled, injecting texas init container")
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, securityConfigForPod.TexasContainer)

		podlog.Info("Injecting texas url")
		for i := range pod.Spec.Containers {
			if pod.Spec.Containers[i].Name == securityConfigForPod.AppName {
				pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, corev1.EnvVar{
					Name:  config.Get().TexasUrlEnvVarName,
					Value: getTexasUrlEnvVarValue(),
				})
			}
		}
	}

	if securityConfigForPod.SecurityConfig.Spec.Opa != nil && securityConfigForPod.SecurityConfig.Spec.Opa.Enabled {
		// Opa is enabled for this Application
		// We inject an init container with Opa in the pod
		podlog.Info("Opa is enabled, injecting Opa init container and config volume")
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, securityConfigForPod.OpaContainer)
		pod.Spec.Volumes = append(pod.Spec.Volumes, securityConfigForPod.OpaConfigVolume)
		ensureEmptyDirVolume(&pod.Spec, OpaTmpVolumeName)

		podlog.Info("Injecting opa url")
		for i := range pod.Spec.Containers {
			if pod.Spec.Containers[i].Name == securityConfigForPod.AppName {
				pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, corev1.EnvVar{
					Name:  config.Get().OpaUrlEnvVarName,
					Value: getOpaUrlEnvVarValue(),
				})
			}
		}
	}

	return nil
}

// +kubebuilder:webhook:path=/validate--v1-pod,mutating=false,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create,versions=v1,name=vpod-v1.kb.io,admissionReviewVersions=v1

// PodCustomValidator struct is responsible for validating the Pod resource
// when it is created, updated, or deleted.
type PodCustomValidator struct {
	Client client.Client
}

var _ webhook.CustomValidator = &PodCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Pod.
func (v *PodCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return validatePod(ctx, v.Client, obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Pod.
func (v *PodCustomValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	return validatePod(ctx, v.Client, newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Pod.
func (v *PodCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("expected a Pod object but got %T", obj)
	}
	podlog.Info("Validation for Pod upon deletion", "name", pod.GetName())

	// Nothing to do

	return nil, nil
}

type PodSecurityConfiguration struct {
	SecurityConfig  *v1alpha.SecurityConfig
	AppName         string
	SecurityEnabled bool
	TexasContainer  corev1.Container
	OpaContainer    corev1.Container
	OpaConfigVolume corev1.Volume
}

// getSecurityConfigForPod extracts the SecurityConfig for a given pod and determines if security is enabled.
// Returns PodSecurityConfiguration with SecurityEnabled=false if security is not enabled or not applicable.
// Returns an error if validation fails (e.g., missing SecurityConfig when security label is present).
func getSecurityConfigForPod(ctx context.Context, crudClient client.Client, pod *corev1.Pod) (*PodSecurityConfiguration, error) {
	if pod.Labels == nil {
		return &PodSecurityConfiguration{SecurityEnabled: false}, nil
	}
	appName, appNameExists := pod.Labels[SkiperatorApplicationRefLabel]
	if !appNameExists {
		return &PodSecurityConfiguration{SecurityEnabled: false}, nil
	}

	if crudClient == nil {
		return nil, fmt.Errorf("webhook client is not configured")
	}

	var skiperatorApplication v1alpha1.Application
	podlog.Info("Fetching Application resource", "name", appName)
	if err := crudClient.Get(ctx, types.NamespacedName{
		Name:      appName,
		Namespace: pod.Namespace,
	}, &skiperatorApplication); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("no Application found with the name %s/%s: %w", pod.Namespace, appName, err)
		}
		return nil, fmt.Errorf("failed to fetch Application resource named %s/%s: %w", pod.Namespace, appName, err)
	}

	if skiperatorApplication.Labels[SecurityEnabledLabelName] != SecurityEnabledLabelValue {
		return &PodSecurityConfiguration{
			AppName:         appName,
			SecurityEnabled: false,
		}, nil
	}

	var securityConfigList v1alpha.SecurityConfigList
	podlog.Info("Fetching SecurityConfig resources")
	if err := crudClient.List(ctx, &securityConfigList, client.InNamespace(pod.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to fetch SecurityConfig resources: %w", err)
	}

	var securityConfigForApplication []v1alpha.SecurityConfig
	for _, securityConfig := range securityConfigList.Items {
		if securityConfig.Spec.ApplicationRef == appName {
			securityConfigForApplication = append(securityConfigForApplication, securityConfig)
		}
	}

	if len(securityConfigForApplication) < 1 {
		msg := fmt.Sprintf(
			"the application is labelled with %s=%s but no SecurityConfig resource was found for Application",
			SecurityEnabledLabelName,
			SecurityEnabledLabelValue,
		)
		podlog.Info(msg, "name", appName)
		return nil, fmt.Errorf("%s", msg)
	}

	if len(securityConfigForApplication) > 1 {
		msg := "multiple SecurityConfig resources found for Application"
		podlog.Info(msg, "name", appName)
		return nil, fmt.Errorf("%s", msg)
	}

	securityConfig := &securityConfigForApplication[0]

	if securityConfig == nil {
		msg := "SecurityConfig resource for Application was nil"
		podlog.Info(msg, "name", appName)
		return nil, fmt.Errorf("%s", msg)
	}

	texasContainer, err := getTexasContainer(*securityConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to construct Texas container: %w", err)
	}
	opaContainer := getOpaContainer(*securityConfig)
	opaConfigVolume := getOpaConfigVolume(*securityConfig)

	return &PodSecurityConfiguration{
		SecurityConfig:  securityConfig,
		AppName:         appName,
		SecurityEnabled: true,
		TexasContainer:  *texasContainer,
		OpaContainer:    opaContainer,
		OpaConfigVolume: opaConfigVolume,
	}, nil
}

func getOpaConfigVolume(securityConfig v1alpha.SecurityConfig) corev1.Volume {
	expectedOpaConfigName := utilities.GetOpaConfigName(securityConfig.Spec.ApplicationRef)

	return corev1.Volume{
		Name: expectedOpaConfigName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: expectedOpaConfigName,
				},
				Items: []corev1.KeyToPath{
					{Key: utilities.OpaConfigFileName, Path: utilities.OpaConfigFileName},
				},
			},
		},
	}
}

func getOpaContainer(securityConfig v1alpha.SecurityConfig) corev1.Container {
	if securityConfig.Spec.Opa == nil || !securityConfig.Spec.Opa.Enabled {
		return corev1.Container{}
	}

	opaImageUrl := fmt.Sprintf(
		"%s:%s",
		config.Get().OpaImageName,
		config.Get().OpaImageTag,
	)

	expectedOpaConfigName := utilities.GetOpaConfigName(securityConfig.Spec.ApplicationRef)
	opaConfigFilePath := OpaConfigMountPath + "/" + utilities.OpaConfigFileName

	return corev1.Container{
		Name:  OpaInitContainerName,
		Image: opaImageUrl,
		Args: []string{
			"run",
			"--server",
			"--config-file=" + opaConfigFilePath,
			"--addr=0.0.0.0:" + strconv.FormatInt(int64(config.Get().OpaPort), 10),
		},
		Ports: []corev1.ContainerPort{
			{Name: OpaPortName, ContainerPort: config.Get().OpaPort, Protocol: corev1.ProtocolTCP},
			{Name: "grpc", ContainerPort: 9191, Protocol: corev1.ProtocolTCP},
		},
		// NOTE: RestartPolicy Always is only available for init containers in Kubernetes v1.33+
		// https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#detailed-behavior
		RestartPolicy: utilities.Ptr(corev1.ContainerRestartPolicyAlways),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: utilities.Ptr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
				Add: []corev1.Capability{
					"NET_BIND_SERVICE",
				},
			},
			Privileged:             utilities.Ptr(false),
			ReadOnlyRootFilesystem: utilities.Ptr(true), // will make tmp/opa writes difficult...
			RunAsGroup:             utilities.Ptr(int64(150)),
			RunAsNonRoot:           utilities.Ptr(true),
			RunAsUser:              utilities.Ptr(int64(150)),
		},
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      expectedOpaConfigName,
				MountPath: OpaConfigMountPath,
				ReadOnly:  true,
			},
			{
				Name:      OpaTmpVolumeName,
				MountPath: "/tmp",
			},
		},
		Env: []corev1.EnvVar{
			{
				Name: utilities.OpaGithubTokenEnvVar,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &securityConfig.Spec.Opa.GithubToken,
				},
			},
			{
				Name: utilities.OpaPublicKeyEnvVar,
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &securityConfig.Spec.Opa.BundlePublicKey,
				},
			},
			{
				Name:  OpaEnabledEnvVarName,
				Value: "true",
			},
		},
	}
}

func getTexasContainer(securityConfig v1alpha.SecurityConfig) (*corev1.Container, error) {
	if securityConfig.Spec.Tokenx == nil || !securityConfig.Spec.Tokenx.Enabled {
		return nil, fmt.Errorf("a texas container should not be created if tokenx is not enabled")
	}
	texasImageUrl := fmt.Sprintf(
		"%s:%s",
		config.Get().TexasImageName,
		config.Get().TexasImageTag,
	)
	expectedJwkerSecretName := utilities.GetJwkerSecretName(
		utilities.GetJwkerName(securityConfig.Spec.ApplicationRef),
	)

	return &corev1.Container{
		Name:  TexasInitContainerName,
		Image: texasImageUrl,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: config.Get().TexasPort,
				Name:          TexasPortName,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		// NOTE: RestartPolicy Always is only available for init containers in Kubernetes v1.33+
		// https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#detailed-behavior
		RestartPolicy: utilities.Ptr(corev1.ContainerRestartPolicyAlways),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: utilities.Ptr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
				Add: []corev1.Capability{
					"NET_BIND_SERVICE",
				},
			},
			Privileged:             utilities.Ptr(false),
			ReadOnlyRootFilesystem: utilities.Ptr(true),
			RunAsGroup:             utilities.Ptr(int64(150)),
			RunAsNonRoot:           utilities.Ptr(true),
			RunAsUser:              utilities.Ptr(int64(150)),
		},
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		Env: []corev1.EnvVar{
			{
				Name:  TokenXEnabledEnvVarName,
				Value: "true",
			},
			{
				Name:  MaskinportenEnabledEnvVarName,
				Value: "false",
			},
			{
				Name:  AzureEnabledEnvVarName,
				Value: "false",
			},
			{
				Name:  IdportenEnabledEnvVarName,
				Value: "false",
			},
		},
		EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: expectedJwkerSecretName}}}},
	}, nil
}

func ensureEmptyDirVolume(podSpec *corev1.PodSpec, name string) {
	for _, volume := range podSpec.Volumes {
		if volume.Name == name {
			return
		}
	}
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
}

func validatePod(ctx context.Context, crudClient client.Client, obj runtime.Object) (admission.Warnings, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("expected an Pod object but got %T", obj)
	}

	podlog.Info("Validating for Pod", "name", pod.GetName())

	securityConfigForPod, getSecurityConfigForPodErr := getSecurityConfigForPod(ctx, crudClient, pod)
	if getSecurityConfigForPodErr != nil {
		podlog.Error(getSecurityConfigForPodErr, "Failed to validate for Pod")
		return nil, getSecurityConfigForPodErr
	}
	if !securityConfigForPod.SecurityEnabled {
		return nil, nil
	}

	if securityConfigForPod.SecurityConfig.Spec.Tokenx != nil && securityConfigForPod.SecurityConfig.Spec.Tokenx.Enabled {
		validateTokenXConfErr := validateTokenxCorrectlyConfigured(pod, securityConfigForPod)
		if validateTokenXConfErr != nil {
			podlog.Error(validateTokenXConfErr, "Failed to validate for Pod")
			return nil, validateTokenXConfErr
		}
	}

	if securityConfigForPod.SecurityConfig.Spec.Opa != nil && securityConfigForPod.SecurityConfig.Spec.Opa.Enabled {
		validateOpaConfErr := validateOpaCorrectlyConfigured(pod, securityConfigForPod)
		if validateOpaConfErr != nil {
			podlog.Error(validateOpaConfErr, "Failed to validate for Pod")
			return nil, validateOpaConfErr
		}
	}

	return nil, nil
}

func validateTokenxCorrectlyConfigured(pod *corev1.Pod, securityConfigForPod *PodSecurityConfiguration) error {
	// Validate that the Texas init container exists
	hasTexasInitContainer := false
	for _, initContainer := range pod.Spec.InitContainers {
		if initContainer.Name == TexasInitContainerName {
			hasTexasInitContainer = true
			if !isTexasContainerEqual(
				securityConfigForPod.TexasContainer,
				initContainer,
			) {
				return fmt.Errorf("texas init container is not as expected given the SecurityConfig")
			}
			break
		}
	}
	if !hasTexasInitContainer {
		podlog.Info("TokenX is enabled but texas init container is missing")
		return fmt.Errorf("TokenX is enabled but init container '%s' is missing", TexasInitContainerName)
	}

	// Validate that the application container has the TEXAS_URL env variable
	hasTexasUrlEnvVar := false
	for _, container := range pod.Spec.Containers {
		if container.Name == securityConfigForPod.AppName {
			for _, envVar := range container.Env {
				if envVar.Name == config.Get().TexasUrlEnvVarName && envVar.Value == getTexasUrlEnvVarValue() {
					hasTexasUrlEnvVar = true
					break
				}
			}
			break
		}
	}
	if !hasTexasUrlEnvVar {
		errMsg := fmt.Sprintf(
			"TokenX is enabled but %s env var is missing for pod from skiperator app with name %s/%s",
			pod.Namespace,
			securityConfigForPod.AppName,
			config.Get().TexasUrlEnvVarName,
		)
		podlog.Info(errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}

func validateOpaCorrectlyConfigured(pod *corev1.Pod, securityConfigForPod *PodSecurityConfiguration) error {
	// Validate that the Opa init container exists
	hasOpaInitContainer := false
	for _, initContainer := range pod.Spec.InitContainers {
		if initContainer.Name == OpaInitContainerName {
			hasOpaInitContainer = true
			if !isOpaContainerEqual(
				securityConfigForPod.OpaContainer,
				initContainer,
			) {
				return fmt.Errorf("Opa init container is not as expected given the SecurityConfig")
			}
			break
		}
	}
	if !hasOpaInitContainer {
		podlog.Info("Opa is enabled but Opa init container is missing")
		return fmt.Errorf("Opa is enabled but init container '%s' is missing", OpaInitContainerName)
	}

	// Validate that the application container has the OPA_URL env variable
	hasOpaUrlEnvVar := false
	for _, container := range pod.Spec.Containers {
		if container.Name == securityConfigForPod.AppName {
			for _, envVar := range container.Env {
				if envVar.Name == config.Get().OpaUrlEnvVarName && envVar.Value == getOpaUrlEnvVarValue() {
					hasOpaUrlEnvVar = true
					break
				}
			}
			break
		}
	}
	if !hasOpaUrlEnvVar {
		errMsg := fmt.Sprintf(
			"Opa is enabled but %s env var is missing for pod from skiperator app with name %s/%s",
			pod.Namespace,
			securityConfigForPod.AppName,
			config.Get().OpaUrlEnvVarName,
		)
		podlog.Info(errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}

func isTexasContainerEqual(expected, actual corev1.Container) bool {
	return expected.Name == actual.Name &&
		expected.Image == actual.Image &&
		reflect.DeepEqual(expected.RestartPolicy, actual.RestartPolicy) &&
		reflect.DeepEqual(expected.Env, actual.Env) &&
		reflect.DeepEqual(expected.EnvFrom, actual.EnvFrom) &&
		reflect.DeepEqual(expected.Ports, actual.Ports) &&
		reflect.DeepEqual(expected.SecurityContext, actual.SecurityContext) &&
		reflect.DeepEqual(expected.TerminationMessagePath, actual.TerminationMessagePath) &&
		reflect.DeepEqual(expected.TerminationMessagePolicy, actual.TerminationMessagePolicy)
}

func isOpaContainerEqual(expected, actual corev1.Container) bool {
	return expected.Name == actual.Name &&
		expected.Image == actual.Image &&
		reflect.DeepEqual(expected.Args, actual.Args) &&
		// Other mutating webhooks inject VolumeMounts into PodSpec
		// reflect.DeepEqual(expected.VolumeMounts, actual.VolumeMounts) &&
		reflect.DeepEqual(expected.RestartPolicy, actual.RestartPolicy) &&
		reflect.DeepEqual(expected.Env, actual.Env) &&
		reflect.DeepEqual(expected.Ports, actual.Ports) &&
		reflect.DeepEqual(expected.SecurityContext, actual.SecurityContext) &&
		reflect.DeepEqual(expected.TerminationMessagePath, actual.TerminationMessagePath) &&
		reflect.DeepEqual(expected.TerminationMessagePolicy, actual.TerminationMessagePolicy)
}

func getTexasUrlEnvVarValue() string {
	return fmt.Sprintf("http://localhost:%d", config.Get().TexasPort)
}

func getOpaUrlEnvVarValue() string {
	return fmt.Sprintf("http://localhost:%d", config.Get().OpaPort)
}
