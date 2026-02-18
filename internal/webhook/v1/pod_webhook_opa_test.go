package v1

import (
	"context"
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/skiperator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("pod_webhook.go unit tests", func() {
	var (
		ctx    context.Context
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(v1alpha.AddToScheme(scheme)).To(Succeed())
	})

	Describe("SetupPodWebhookWithManager", func() {
		It("panics when manager is nil (sanity coverage)", func() {
			// This is a lightweight coverage test. Proper webhook wiring is validated via envtest/chainsaw.
			Expect(func() { _ = SetupPodWebhookWithManager(ctrl.Manager(nil)) }).To(Panic())
		})
	})

	Describe("getOpaUrlEnvVarValue", func() {
		It("returns a localhost URL including the configured port", func() {
			v := getOpaUrlEnvVarValue()
			Expect(v).To(ContainSubstring("http://localhost:"))
			Expect(v).To(ContainSubstring(fmt.Sprintf(":%d", config.Get().OpaPort)))
		})
	})

	Describe("getOpaContainer", func() {
		It("returns empty container when opa is not enabled", func() {
			applicationRef := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					Opa: &v1alpha.OpaSpec{
						Enabled: false,
					},
					ApplicationRef: applicationRef,
				},
			}
			c, err := getOpaContainer(securityConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(*c).To(Equal(corev1.Container{}))
		})

		It("builds a opa init container with expected basic properties", func() {
			applicationRef := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					Opa: &v1alpha.OpaSpec{
						Enabled: true,
					},
					ApplicationRef: applicationRef,
				},
			}
			c, err := getOpaContainer(securityConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Image).To(Equal(fmt.Sprintf("%s:%s", config.Get().OpaImageName, config.Get().OpaImageTag)))
			Expect(*c.RestartPolicy).To(Equal(corev1.ContainerRestartPolicyAlways))
			Expect(c.SecurityContext).ToNot(BeNil())
			Expect(c.Env).NotTo(BeEmpty())
			Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: OpaEnabledEnvVarName, Value: "true"}))
			// Check for github token and opa public sign key
		})
	})

	Describe("isOpaContainerEqual", func() {
		It("returns true for identical containers and false when a field differs", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					Opa: &v1alpha.OpaSpec{
						Enabled: true,
					},
					ApplicationRef: "myapp",
				}}
			a, errA := getOpaContainer(securityConfig)
			Expect(errA).ToNot(HaveOccurred())
			b, errB := getOpaContainer(securityConfig)
			Expect(errB).ToNot(HaveOccurred())
			Expect(isOpaContainerEqual(
				*a, *b)).To(BeTrue())

			b.Image = b.Image + "-changed"
			Expect(isOpaContainerEqual(*a, *b)).To(BeFalse())
		})
	})

	Describe("getSecurityConfigForPod", func() {
		It("returns full PodSecurityConfiguration when pod is created by Skiperator Application with correct label and when a SecurityConfig referencing the same app exists", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
			}

			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: pod.Namespace,
				},
				Spec: v1alpha.SecurityConfigSpec{
					Opa:            &v1alpha.OpaSpec{Enabled: true},
					Tokenx:         &v1alpha.TokenXSpec{Enabled: true},
					ApplicationRef: skiperatorAppName,
				},
			}

			cfg, err := getSecurityConfigForPod(
				ctx,
				utilities.GetMockKubernetesClient(
					scheme,
					&v1alpha1.Application{
						ObjectMeta: metav1.ObjectMeta{
							Name:      skiperatorAppName,
							Namespace: pod.Namespace,
							Labels: map[string]string{
								SecurityEnabledLabelName: SecurityEnabledLabelValue,
							},
						},
					},
					&securityConfig,
				),
				pod,
			)

			// Crazy life: Tokenx must be enabled or get securityConfig will return error
			texasContainer, getTexasErr := getTexasContainer(securityConfig)
			Expect(getTexasErr).ToNot(HaveOccurred())
			opaContainer, getOpaErr := getOpaContainer(securityConfig)
			opaVolume, getOpaVolumeErr := getOpaConfigVolume(securityConfig)
			Expect(getOpaVolumeErr).ToNot(HaveOccurred())
			Expect(getOpaErr).ToNot(HaveOccurred())
			Expect(err).ToNot(HaveOccurred())
			Expect(*cfg).To(Equal(
				PodSecurityConfiguration{
					SecurityConfig:  &securityConfig,
					AppName:         skiperatorAppName,
					SecurityEnabled: true,
					TexasContainer:  *texasContainer,
					OpaContainer:    *opaContainer,
					OpaConfigVolume: *opaVolume,
				},
			))
		})
	})

	Describe("isOpaContainerEqual", func() {
		It("returns true when two opa containers are equal on the fields that are used", func() {
			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: "ns",
				},
				Spec: v1alpha.SecurityConfigSpec{
					Opa: &v1alpha.OpaSpec{
						Enabled: true,
					},
					ApplicationRef: "my-app",
				},
			}
			opaContainer, _ := getOpaContainer(securityConfig)
			result := isOpaContainerEqual(
				*opaContainer,
				*opaContainer,
			)
			Expect(result).To(BeTrue())
		})

		It("returns false when two opa containers are not equal on the fields that are used", func() {
			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: "ns",
				},
				Spec: v1alpha.SecurityConfigSpec{
					Opa: &v1alpha.OpaSpec{
						Enabled: true,
					},
					ApplicationRef: "my-app",
				},
			}
			opaContainer, _ := getOpaContainer(securityConfig)
			alteredOpaContainer := *opaContainer
			alteredOpaContainer.Ports = append(
				alteredOpaContainer.Ports,
				corev1.ContainerPort{
					Name:          "dummy-port",
					ContainerPort: 1234,
					Protocol:      "UDP",
				},
			)
			result := isOpaContainerEqual(
				*opaContainer,
				alteredOpaContainer,
			)
			Expect(result).To(BeFalse())
		})
	})

	Describe("validateOpaCorrectlyConfigured", func() {
		It("returns error when pod should have opa init container but it does not have opa init container", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{},
				},
			}

			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: pod.Namespace,
				},
				Spec: v1alpha.SecurityConfigSpec{
					Opa:            &v1alpha.OpaSpec{Enabled: true},
					ApplicationRef: skiperatorAppName,
				},
			}

			opaContainer, getOpaErr := getOpaContainer(securityConfig)
			Expect(getOpaErr).ToNot(HaveOccurred())

			podSecurityConfig := PodSecurityConfiguration{
				SecurityConfig:  &securityConfig,
				AppName:         skiperatorAppName,
				SecurityEnabled: true,
				OpaContainer:    *opaContainer,
			}

			validateOpaErr := validateOpaCorrectlyConfigured(pod, &podSecurityConfig)
			Expect(validateOpaErr).To(
				MatchError(
					Equal(
						fmt.Sprintf("Opa is enabled but init container '%s' is missing", OpaInitContainerName),
					),
				),
			)
		})

		It("returns error when Opa is enabled, opa init container is injected but opa environment variable is missing from main app container", func() {
			skiperatorAppName := skiperatorAppName
			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: "ns",
				},
				Spec: v1alpha.SecurityConfigSpec{
					Opa:            &v1alpha.OpaSpec{Enabled: true},
					ApplicationRef: skiperatorAppName,
				},
			}
			opaContainer, getOpaErr := getOpaContainer(securityConfig)
			Expect(getOpaErr).ToNot(HaveOccurred())

			podSecurityConfig := PodSecurityConfiguration{
				SecurityConfig:  &securityConfig,
				AppName:         skiperatorAppName,
				SecurityEnabled: true,
				OpaContainer:    *opaContainer,
			}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: securityConfig.Namespace,
					Labels: map[string]string{
						SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{*opaContainer},
				},
			}

			validateOpaErr := validateOpaCorrectlyConfigured(pod, &podSecurityConfig)
			Expect(validateOpaErr).To(
				MatchError(
					Equal(
						fmt.Sprintf(
							"Opa is enabled but %s env var is missing for pod from skiperator app with name %s/%s",
							pod.Namespace,
							podSecurityConfig.AppName,
							config.Get().OpaUrlEnvVarName,
						),
					),
				),
			)
		})

		It("returns no error when Opa is enabled, correct Opa container is injected and opa env var is injected in main app container", func() {
			skiperatorAppName := skiperatorAppName
			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: "ns",
				},
				Spec: v1alpha.SecurityConfigSpec{
					Opa:            &v1alpha.OpaSpec{Enabled: true},
					ApplicationRef: skiperatorAppName,
				},
			}
			opaContainer, getOpaErr := getOpaContainer(securityConfig)
			Expect(getOpaErr).ToNot(HaveOccurred())

			podSecurityConfig := PodSecurityConfiguration{
				SecurityConfig:  &securityConfig,
				AppName:         skiperatorAppName,
				SecurityEnabled: true,
				OpaContainer:    *opaContainer,
			}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: securityConfig.Namespace,
					Labels: map[string]string{
						SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{*opaContainer},
					Containers: []corev1.Container{
						{
							Name: podSecurityConfig.AppName,
							Env: []corev1.EnvVar{
								{
									Name:  config.Get().OpaUrlEnvVarName,
									Value: getOpaUrlEnvVarValue(),
								},
							},
						},
					},
				},
			}

			validateOpaErr := validateOpaCorrectlyConfigured(pod, &podSecurityConfig)
			Expect(validateOpaErr).ToNot(HaveOccurred())
		})
	})
})
