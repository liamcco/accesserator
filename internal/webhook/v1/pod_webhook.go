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

	"github.com/kartverket/accesserator/api/v1alpha"
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

// nolint:unused
// log is for logging in this package.
var podlog = logf.Log.WithName("pod-resource")

// SetupPodWebhookWithManager registers the webhook for Pod in the manager.
func SetupPodWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&corev1.Pod{}).
		WithValidator(&PodCustomValidator{}).
		WithDefaulter(&PodCustomDefaulter{Client: mgr.GetClient()}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

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
	podlog.Info("Defaulting for Pod", "name", pod.GetName())

	if pod.Labels == nil {
		// Nothing to mutate
		return nil
	}

	appName, appNameExists := pod.Labels["application.skiperator.no/app-name"]
	if !appNameExists {
		// Nothing to mutate
		return nil
	}

	// Fetch the resource named by appName. (Placeholder type until Skiperator Application API is wired in)
	if d.Client == nil {
		return fmt.Errorf("webhook client is not configured")
	}

	var skiperatorApplication v1alpha1.Application
	podlog.Info("Fetching Application resource", "name", appName)
	if exists := d.Client.Get(ctx, types.NamespacedName{
		Name:      appName,
		Namespace: pod.Namespace,
	}, &skiperatorApplication); exists != nil {
		return fmt.Errorf("failed to fetch Application resource named %s: %w", appName, exists)
	}

	// Check if Application has correct label
	if skiperatorApplication.Labels["skiperator/security"] != "enabled" {
		// Nothing to mutate
		return nil
	}

	var securityConfigList v1alpha.SecurityConfigList
	podlog.Info("Fetching SecurityConfig resources")
	if securityConfigListErr := d.Client.List(ctx, &securityConfigList, client.InNamespace(pod.Namespace)); securityConfigListErr != nil {
		return fmt.Errorf("failed to fetch SecurityConfig resources %w", securityConfigListErr)
	}

	var securityConfigForApplication []v1alpha.SecurityConfig
	for _, securityConfig := range securityConfigList.Items {
		if securityConfig.Spec.ApplicationRef == appName {
			securityConfigForApplication = append(securityConfigForApplication, securityConfig)
		}
	}
	if len(securityConfigForApplication) < 1 {
		// This is an unwanted state because the Application is labeled with
		// the label skiperator/security=enabled, so validating webhook will fail for this case.

		message := `the application is labelled with skiperator/security=enabled 
		but no SecurityConfig resource was found for Application`
		podlog.Info(message, "name", appName)
		return nil
	} else if len(securityConfigForApplication) > 1 {
		// This is an unwanted state because multiple SecurityConfigs cannot target the same Application. Validating webhook will fail for this case.
		podlog.Info("multiple SecurityConfig resources found for Application", "name", appName)
	}
	securityConfig := securityConfigForApplication[0]

	if securityConfig.Spec.Tokenx != nil && securityConfig.Spec.Tokenx.Enabled {
		// TokenX is enabled for this Application
		// We inject an init container with texas in the pod
		expectedJwkerSecretName := fmt.Sprintf("%s-jwker-secret", appName)
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:  "texas",
			Image: "ghcr.io/nais/texas:latest",
			Ports: []corev1.ContainerPort{{ContainerPort: 3000}},
			// NOTE: RestartPolicy Always is only avaiable for init containers in Kubernetes v1.33+
			// https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#detailed-behavior
			RestartPolicy: utilities.Ptr(corev1.ContainerRestartPolicyAlways),
			Env: []corev1.EnvVar{
				{
					Name:  "TOKEN_X_ENABLED",
					Value: "true",
				},
				{
					Name:  "MASKINPORTEN_ENABLED",
					Value: "false",
				},
				{
					Name:  "AZURE_ENABLED",
					Value: "false",
				},
				{
					Name:  "IDPORTEN_ENABLED",
					Value: "false",
				},
			},
			EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: expectedJwkerSecretName}}}},
		})

		for i := range pod.Spec.Containers {
			if pod.Spec.Containers[i].Name == appName {
				pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, corev1.EnvVar{
					Name:  "TEXAS_URL",
					Value: "http://localhost:3000",
				})
			}
		}
	}

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate--v1-pod,mutating=false,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create,versions=v1,name=vpod-v1.kb.io,admissionReviewVersions=v1

// PodCustomValidator struct is responsible for validating the Pod resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type PodCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &PodCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Pod.
func (v *PodCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("expected a Pod object but got %T", obj)
	}
	podlog.Info("Validation for Pod upon creation", "name", pod.GetName())

	// TODO: Implement validation logic for Pod creation

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Pod.
func (v *PodCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	pod, ok := newObj.(*corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("expected a Pod object for the newObj but got %T", newObj)
	}
	podlog.Info("Validation for Pod upon update", "name", pod.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Pod.
func (v *PodCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("expected a Pod object but got %T", obj)
	}
	podlog.Info("Validation for Pod upon deletion", "name", pod.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
