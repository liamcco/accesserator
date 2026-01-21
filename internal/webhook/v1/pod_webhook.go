/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"
	"fmt"
	"reflect"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/skiperator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	TexasInitContainerName = "texas"
	TexasPortName          = "http"

	MaskinportenEnabledEnvVarName = "MASKINPORTEN_ENABLED"
	AzureEnabledEnvVarName        = "AZURE_ENABLED"
	IdportenEnabledEnvVarName     = "IDPORTEN_ENABLED"
	TokenXEnabledEnvVarName       = "TOKEN_X_ENABLED"
)

// nolint:unused
// log is for logging in this package.
var podlog = logf.Log.WithName("pod-resource")

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

	return nil
}

// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate--v1-pod,mutating=false,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create,versions=v1,name=vpod-v1.kb.io,admissionReviewVersions=v1

// PodCustomValidator struct is responsible for validating the Pod resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type PodCustomValidator struct {
	Client client.Client
}

var _ webhook.CustomValidator = &PodCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Pod.
func (v *PodCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return validatePod(ctx, v.Client, obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Pod.
func (v *PodCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return validatePod(ctx, v.Client, newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Pod.
func (v *PodCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
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
}

// getSecurityConfigForPod extracts the SecurityConfig for a given pod and determines if security is enabled.
// Returns PodSecurityConfiguration with SecurityEnabled=false if security is not enabled or not applicable.
// Returns an error if validation fails (e.g., missing SecurityConfig when security label is present).
func getSecurityConfigForPod(ctx context.Context, crudClient client.Client, pod *corev1.Pod) (*PodSecurityConfiguration, error) {
	if pod.Labels == nil {
		return &PodSecurityConfiguration{SecurityEnabled: false}, nil
	}
	appName, appNameExists := pod.Labels["application.skiperator.no/app-name"]
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
		return nil, fmt.Errorf("failed to fetch Application resource named %s: %w", appName, err)
	}

	if skiperatorApplication.Labels["skiperator/security"] != "enabled" {
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
		msg := "the application is labelled with skiperator/security=enabled but no SecurityConfig resource was found for Application"
		podlog.Info(msg, "name", appName)
		return nil, fmt.Errorf("%s", msg)
	}

	if len(securityConfigForApplication) > 1 {
		msg := "multiple SecurityConfig resources found for Application"
		podlog.Info(msg, "name", appName)
		return nil, fmt.Errorf("%s", msg)
	}

	securityConfig := &securityConfigForApplication[0]

	texasContainer := getTexasContainer(securityConfig.Name)

	return &PodSecurityConfiguration{
		SecurityConfig:  securityConfig,
		AppName:         appName,
		SecurityEnabled: true,
		TexasContainer:  texasContainer,
	}, nil
}

func getTexasContainer(securityConfigName string) corev1.Container {
	texasImageUrl := fmt.Sprintf(
		"%s:%s",
		config.Get().TexasImageName,
		config.Get().TexasImageTag,
	)
	expectedJwkerSecretName := utilities.GetJwkerSecretName(
		utilities.GetJwkerName(securityConfigName),
	)

	return corev1.Container{
		Name:  TexasInitContainerName,
		Image: texasImageUrl,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: config.Get().TexasPort,
				Name:          TexasPortName,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		// NOTE: RestartPolicy Always is only avaiable for init containers in Kubernetes v1.33+
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
	}
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
		warnings, validateTokenXConfErr := validateTokenxCorrectlyConfigured(pod, securityConfigForPod)
		if validateTokenXConfErr != nil {
			podlog.Error(validateTokenXConfErr, "Failed to validate for Pod")
			return warnings, validateTokenXConfErr
		}
	}

	return nil, nil
}

func validateTokenxCorrectlyConfigured(pod *corev1.Pod, securityConfigForPod *PodSecurityConfiguration) (admission.Warnings, error) {
	// Validate that the Texas init container exists
	hasTexasInitContainer := false
	for _, initContainer := range pod.Spec.InitContainers {
		if initContainer.Name == TexasInitContainerName {
			hasTexasInitContainer = true
			if !isTexasContainerEqual(
				securityConfigForPod.TexasContainer,
				initContainer,
			) {
				return nil, fmt.Errorf("texas init container is not as expected given the SecurityConfig")
			}
			break
		}
	}
	if !hasTexasInitContainer {
		podlog.Info("TokenX is enabled but texas init container is missing")
		return nil, fmt.Errorf("TokenX is enabled but init container '%s' is missing", TexasInitContainerName)
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
		podlog.Info("TokenX is enabled but TEXAS_URL env var is missing", "container", securityConfigForPod.AppName)
		return nil, fmt.Errorf("TokenX is enabled but container '%s' is missing environment variable '%s'", securityConfigForPod.AppName, config.Get().TexasUrlEnvVarName)
	}
	return nil, nil
}

func isTexasContainerEqual(expected, actual corev1.Container) bool {
	isEqual := true
	if expected.Name != actual.Name ||
		expected.Image != actual.Image ||
		!reflect.DeepEqual(expected.RestartPolicy, actual.RestartPolicy) ||
		!reflect.DeepEqual(expected.Env, actual.Env) ||
		!reflect.DeepEqual(expected.EnvFrom, actual.EnvFrom) ||
		!reflect.DeepEqual(expected.Ports, actual.Ports) ||
		!reflect.DeepEqual(expected.SecurityContext, actual.SecurityContext) ||
		!reflect.DeepEqual(expected.TerminationMessagePath, actual.TerminationMessagePath) ||
		!reflect.DeepEqual(expected.TerminationMessagePolicy, actual.TerminationMessagePolicy) {
		isEqual = false
	}
	return isEqual
}

func getTexasUrlEnvVarValue() string {
	return fmt.Sprintf("http://localhost:%d", config.Get().TexasPort)
}
