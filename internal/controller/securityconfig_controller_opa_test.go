package controller

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"

	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/skiperator/api/v1alpha1"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

var _ = Describe("SecurityConfig Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-resource"
			skiperatorAppName = "test-app"
			namespaceName     = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: namespaceName,
		}
		securityConfig := &accesseratorv1alpha.SecurityConfig{}
		application := &v1alpha1.Application{}

		BeforeEach(func() {
			By("creating the dependent Application custom resource")
			appKey := types.NamespacedName{Name: skiperatorAppName, Namespace: "default"}
			err := k8sClient.Get(ctx, appKey, application)
			if err != nil && errors.IsNotFound(err) {
				app := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      skiperatorAppName,
						Namespace: namespaceName,
					},
					Spec: v1alpha1.ApplicationSpec{
						AccessPolicy: &podtypes.AccessPolicy{},
					},
				}
				Expect(k8sClient.Create(ctx, app)).To(Succeed())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			By("creating the custom resource for the Kind SecurityConfig")
			err = k8sClient.Get(ctx, typeNamespacedName, securityConfig)
			if err != nil && errors.IsNotFound(err) {
				securityConfig := &accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespaceName,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: skiperatorAppName,
						Opa: &accesseratorv1alpha.OpaSpec{
							Enabled:       true,
							BundlePath:    "ghcr.io/kartverket/taaask-poc",
							BundleVersion: "latest",
						},
					},
				}
				Expect(k8sClient.Create(ctx, securityConfig)).To(Succeed())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		})

		AfterEach(func() {
			resource := &accesseratorv1alpha.SecurityConfig{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance SecurityConfig")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("Cleanup any created Jwker resource")
			jwker := &naisiov1.Jwker{}
			jwkerKey := types.NamespacedName{Name: utilities.GetJwkerName(skiperatorAppName), Namespace: namespaceName}
			if err := k8sClient.Get(ctx, jwkerKey, jwker); err == nil {
				Expect(k8sClient.Delete(ctx, jwker)).To(Succeed())
			}

			By("Cleanup any created Opa config")
			opaConfig := &corev1.ConfigMap{}
			opaConfigKey := types.NamespacedName{
				Name:      utilities.GetOpaConfigName(skiperatorAppName),
				Namespace: namespaceName,
			}
			if err := k8sClient.Get(ctx, opaConfigKey, opaConfig); err == nil {
				Expect(k8sClient.Delete(ctx, opaConfig)).To(Succeed())
			}

			By("Cleanup any created Opa discovery config")
			opaDiscoveryConfig := &corev1.ConfigMap{}
			opaDiscoveryConfigKey := types.NamespacedName{
				Name:      utilities.GetOpaDiscoveryConfigName(skiperatorAppName),
				Namespace: namespaceName,
			}
			if err := k8sClient.Get(ctx, opaDiscoveryConfigKey, opaDiscoveryConfig); err == nil {
				Expect(k8sClient.Delete(ctx, opaDiscoveryConfig)).To(Succeed())
			}

			By("Cleanup any created Opa discovery service")
			opaDiscoveryService := &corev1.Service{}
			opaDiscoveryServiceKey := types.NamespacedName{
				Name:      utilities.GetOpaDiscoveryServiceName(skiperatorAppName),
				Namespace: namespaceName,
			}
			if err := k8sClient.Get(ctx, opaDiscoveryServiceKey, opaDiscoveryService); err == nil {
				Expect(k8sClient.Delete(ctx, opaDiscoveryService)).To(Succeed())
			}

			By("Cleanup any created Opa discovery deployment")
			opaDiscoveryDeployment := &appsv1.Deployment{}
			opaDiscoveryDeploymentKey := types.NamespacedName{
				Name:      utilities.GetOpaDiscoveryDeploymentName(skiperatorAppName),
				Namespace: namespaceName,
			}
			if err := k8sClient.Get(ctx, opaDiscoveryDeploymentKey, opaDiscoveryDeployment); err == nil {
				Expect(k8sClient.Delete(ctx, opaDiscoveryDeployment)).To(Succeed())
			}

			By("Cleanup any created Opa discovery egress policy")
			opaDiscoveryEgressPolicy := &networkingv1.NetworkPolicy{}
			opaDiscoveryEgressPolicyKey := types.NamespacedName{
				Name:      utilities.GetOpaDiscoveryEgressPolicyName(skiperatorAppName),
				Namespace: namespaceName,
			}
			if err := k8sClient.Get(ctx, opaDiscoveryEgressPolicyKey, opaDiscoveryEgressPolicy); err == nil {
				Expect(k8sClient.Delete(ctx, opaDiscoveryEgressPolicy)).To(Succeed())
			}

			By("Cleanup any created Opa discovery ingress policy")
			opaDiscoveryIngressPolicy := &networkingv1.NetworkPolicy{}
			opaDiscoveryIngressPolicyKey := types.NamespacedName{
				Name:      utilities.GetOpaDiscoveryIngressPolicyName(skiperatorAppName),
				Namespace: namespaceName,
			}
			if err := k8sClient.Get(ctx, opaDiscoveryIngressPolicyKey, opaDiscoveryIngressPolicy); err == nil {
				Expect(k8sClient.Delete(ctx, opaDiscoveryIngressPolicy)).To(Succeed())
			}

			skiperatorApp := &v1alpha1.Application{}
			appKey := types.NamespacedName{Name: skiperatorAppName, Namespace: namespaceName}
			err = k8sClient.Get(ctx, appKey, skiperatorApp)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the dependant Application resource")
			Expect(k8sClient.Delete(ctx, skiperatorApp)).To(Succeed())
		})

		It("should create a opa-config when Opa is enabled", func() {
			By("Reconciling the SecurityConfig with Opa enabled")

			fakeRecorder := record.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that a opa-config resource was created")
			var opaConfig corev1.ConfigMap
			opaConfigKey := types.NamespacedName{
				Name:      utilities.GetOpaConfigName(skiperatorAppName),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, opaConfigKey, &opaConfig)
			}).Should(Succeed())

			By("Verifying that a Opa discovery ConfigMap was created")
			var opaDiscoveryConfig corev1.ConfigMap
			opaDiscoveryConfigKey := types.NamespacedName{
				Name:      utilities.GetOpaDiscoveryConfigName(skiperatorAppName),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, opaDiscoveryConfigKey, &opaDiscoveryConfig)
			}).Should(Succeed())

			By("Verifying that a Opa discovery Service was created")
			var opaDiscoveryService corev1.Service
			opaDiscoveryServiceKey := types.NamespacedName{
				Name:      utilities.GetOpaDiscoveryServiceName(skiperatorAppName),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, opaDiscoveryServiceKey, &opaDiscoveryService)
			}).Should(Succeed())

			By("Verifying that a Opa discovery Deployment was created")
			var opaDiscoveryDeployment appsv1.Deployment
			opaDiscoveryDeploymentKey := types.NamespacedName{
				Name:      utilities.GetOpaDiscoveryDeploymentName(skiperatorAppName),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, opaDiscoveryDeploymentKey, &opaDiscoveryDeployment)
			}).Should(Succeed())

			By("Verifying that a Opa discovery egress NetworkPolicy was created")
			var opaDiscoveryEgressPolicy networkingv1.NetworkPolicy
			opaDiscoveryEgressPolicyKey := types.NamespacedName{
				Name:      utilities.GetOpaDiscoveryEgressPolicyName(skiperatorAppName),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, opaDiscoveryEgressPolicyKey, &opaDiscoveryEgressPolicy)
			}).Should(Succeed())

			By("Verifying that a Opa discovery ingress NetworkPolicy was created")
			var opaDiscoveryIngressPolicy networkingv1.NetworkPolicy
			opaDiscoveryIngressPolicyKey := types.NamespacedName{
				Name:      utilities.GetOpaDiscoveryIngressPolicyName(skiperatorAppName),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, opaDiscoveryIngressPolicyKey, &opaDiscoveryIngressPolicy)
			}).Should(Succeed())

			By("Verifying that SecurityConfig is PhasePending before OpaConfig is ready")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				sc := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, sc); err != nil {
					return "", err
				}
				return sc.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhaseReady))

			By("Reconciling again to let SecurityConfig transition to PhaseReady")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that SecurityConfig transitioned to PhaseReady")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				sc := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, sc); err != nil {
					return "", err
				}
				return sc.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhaseReady))

			By("Verifying events were emitted")
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconcileStarted")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconciledSuccessfully")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconcileSuccess")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("StatusUpdateSuccess")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("ReconcileFailed")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("StatusUpdateFailed")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("ReconcileFailed")))
		})

		It("should NOT create a OpaConfig resource when Opa is disabled", func() {
			By("Disabling Opa on the SecurityConfig")
			securityConfig := &accesseratorv1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, securityConfig)).To(Succeed())

			securityConfig.Spec.Opa = nil
			Expect(k8sClient.Update(ctx, securityConfig)).To(Succeed())

			By("Reconciling the SecurityConfig with Opa disabled")

			fakeRecorder := record.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that no OpaConfig resource exists")
			var opaConfig corev1.ConfigMap
			Consistently(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.GetOpaConfigName(skiperatorAppName),
					Namespace: namespaceName,
				}, &opaConfig)
				return errors.IsNotFound(err)
			}).Should(BeTrue())

			By("Verifying that no Opa discovery ConfigMap resource exists")
			var opaDiscoveryConfig corev1.ConfigMap
			Consistently(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.GetOpaDiscoveryConfigName(skiperatorAppName),
					Namespace: namespaceName,
				}, &opaDiscoveryConfig)
				return errors.IsNotFound(err)
			}).Should(BeTrue())

			By("Verifying that no Opa discovery Service resource exists")
			var opaDiscoveryService corev1.Service
			Consistently(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.GetOpaDiscoveryServiceName(skiperatorAppName),
					Namespace: namespaceName,
				}, &opaDiscoveryService)
				return errors.IsNotFound(err)
			}).Should(BeTrue())

			By("Verifying that no Opa discovery Deployment resource exists")
			var opaDiscoveryDeployment appsv1.Deployment
			Consistently(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.GetOpaDiscoveryDeploymentName(skiperatorAppName),
					Namespace: namespaceName,
				}, &opaDiscoveryDeployment)
				return errors.IsNotFound(err)
			}).Should(BeTrue())

			By("Verifying that no Opa discovery egress NetworkPolicy resource exists")
			var opaDiscoveryEgressPolicy networkingv1.NetworkPolicy
			Consistently(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.GetOpaDiscoveryEgressPolicyName(skiperatorAppName),
					Namespace: namespaceName,
				}, &opaDiscoveryEgressPolicy)
				return errors.IsNotFound(err)
			}).Should(BeTrue())

			By("Verifying that no Opa discovery ingress NetworkPolicy resource exists")
			var opaDiscoveryIngressPolicy networkingv1.NetworkPolicy
			Consistently(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.GetOpaDiscoveryIngressPolicyName(skiperatorAppName),
					Namespace: namespaceName,
				}, &opaDiscoveryIngressPolicy)
				return errors.IsNotFound(err)
			}).Should(BeTrue())

			By("Verifying events were emitted")
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconcileStarted")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconciledSuccessfully")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconcileSuccess")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("StatusUpdateSuccess")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("ReconcileFailed")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("StatusUpdateFailed")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("ReconcileFailed")))
		})

		It("should update the Opa discovery document when bundle version changes", func() {
			fakeRecorder := record.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)

			By("Reconciling once to create the initial Opa discovery resources")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			opaDiscoveryConfigKey := types.NamespacedName{
				Name:      utilities.GetOpaDiscoveryConfigName(skiperatorAppName),
				Namespace: namespaceName,
			}

			By("Verifying the initial bundle version is present in discovery document")
			var opaDiscoveryConfig corev1.ConfigMap
			Eventually(func() error {
				return k8sClient.Get(ctx, opaDiscoveryConfigKey, &opaDiscoveryConfig)
			}).Should(Succeed())
			initialDiscoveryData, initialDiscoveryErr := extractDiscoveryDataJSON(
				opaDiscoveryConfig.BinaryData[utilities.OpaDiscoveryBundleFileName],
			)
			Expect(initialDiscoveryErr).NotTo(HaveOccurred())
			Expect(initialDiscoveryData).To(
				ContainSubstring("ghcr.io/kartverket/taaask-poc:latest"),
			)

			By("Updating bundle version in SecurityConfig")
			currentSecurityConfig := &accesseratorv1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, currentSecurityConfig)).To(Succeed())
			currentSecurityConfig.Spec.Opa.BundleVersion = "v2.0.0"
			Expect(k8sClient.Update(ctx, currentSecurityConfig)).To(Succeed())

			By("Reconciling again to update discovery document")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying discovery document now contains the updated bundle version")
			Eventually(func() string {
				updatedConfig := &corev1.ConfigMap{}
				if getErr := k8sClient.Get(ctx, opaDiscoveryConfigKey, updatedConfig); getErr != nil {
					return ""
				}
				discoveryDataJSON, discoveryDataErr := extractDiscoveryDataJSON(
					updatedConfig.BinaryData[utilities.OpaDiscoveryBundleFileName],
				)
				if discoveryDataErr != nil {
					return ""
				}
				return discoveryDataJSON
			}).Should(ContainSubstring("ghcr.io/kartverket/taaask-poc:v2.0.0"))
		})
	})
})

func extractDiscoveryDataJSON(bundle []byte) (string, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(bundle))
	if err != nil {
		return "", err
	}
	defer func() { _ = gzipReader.Close() }()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			return "", io.EOF
		}
		if nextErr != nil {
			return "", nextErr
		}
		if header.Name == "data.json" {
			content, readErr := io.ReadAll(tarReader)
			if readErr != nil {
				return "", readErr
			}
			return string(content), nil
		}
	}
}
