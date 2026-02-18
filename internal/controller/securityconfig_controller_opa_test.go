package controller

import (
	"context"

	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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

			By("Verifying events were emitted")
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconcileStarted")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconciledSuccessfully")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("ReconcileSuccess")))
			Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("StatusUpdateSuccess")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("ReconcileFailed")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("StatusUpdateFailed")))
			Eventually(fakeRecorder.Events).ShouldNot(Receive(ContainSubstring("ReconcileFailed")))
		})
	})
})
