package controller

import (
	"context"

	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/skiperator/api/v1alpha1"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

var _ = Describe("SecurityConfig Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			securityConfigName = "test-resource"
			skiperatorAppName  = "test-app"
			namespaceName      = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      securityConfigName,
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
						Name:      securityConfigName,
						Namespace: namespaceName,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: skiperatorAppName,
						Tokenx: &accesseratorv1alpha.TokenXSpec{
							Enabled: true,
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

			By("Cleanup any created Netpol resource")
			netpol := &v1.NetworkPolicy{}
			netpolKey := types.NamespacedName{Name: utilities.GetTokenxEgressName(securityConfigName, config.Get().TokenxName), Namespace: namespaceName}
			if err := k8sClient.Get(ctx, netpolKey, netpol); err == nil {
				Expect(k8sClient.Delete(ctx, netpol)).To(Succeed())
			}

			skiperatorApp := &v1alpha1.Application{}
			appKey := types.NamespacedName{Name: skiperatorAppName, Namespace: namespaceName}
			err = k8sClient.Get(ctx, appKey, skiperatorApp)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the dependant Application resource")
			Expect(k8sClient.Delete(ctx, skiperatorApp)).To(Succeed())
		})

		It("should create a Jwker resource and a NetworkPolicy when TokenX is enabled", func() {
			By("Reconciling the SecurityConfig with TokenX enabled")

			fakeRecorder := record.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that a NetworkPolicy resource was created")
			netpol := &v1.NetworkPolicy{}
			netpolKey := types.NamespacedName{
				Name:      utilities.GetTokenxEgressName(securityConfigName, config.Get().TokenxName),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, netpolKey, netpol)
			}).Should(Succeed())

			By("Verifying that a Jwker resource was created")
			jwker := &naisiov1.Jwker{}
			jwkerKey := types.NamespacedName{
				Name:      utilities.GetJwkerName(skiperatorAppName),
				Namespace: namespaceName,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, jwkerKey, jwker)
			}).Should(Succeed())

			By("Verifying that SecurityConfig is PhasePending before Jwker is ready")
			Eventually(func() (accesseratorv1alpha.Phase, error) {
				sc := &accesseratorv1alpha.SecurityConfig{}
				if err := k8sClient.Get(ctx, typeNamespacedName, sc); err != nil {
					return "", err
				}
				return sc.Status.Phase, nil
			}).Should(Equal(accesseratorv1alpha.PhasePending))

			By("Marking the Jwker resource as finished reconciling")
			jwker.Status.SynchronizationState = jwkerSynchronizationStateReady
			Expect(k8sClient.Status().Update(ctx, jwker)).To(Succeed())

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

		It("should NOT create a Jwker resource nor a NetworkPolicy resource when TokenX is disabled", func() {
			By("Disabling TokenX on the SecurityConfig")
			securityConfig := &accesseratorv1alpha.SecurityConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, securityConfig)).To(Succeed())

			securityConfig.Spec.Tokenx = nil
			Expect(k8sClient.Update(ctx, securityConfig)).To(Succeed())

			By("Reconciling the SecurityConfig with TokenX disabled")

			fakeRecorder := record.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that no Jwker resource exists")
			jwker := &naisiov1.Jwker{}
			Consistently(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.GetJwkerName(skiperatorAppName),
					Namespace: namespaceName,
				}, jwker)
				return errors.IsNotFound(err)
			}).Should(BeTrue())

			By("Verifying that no NetworkPolicy resource exists")
			netpol := &v1.NetworkPolicy{}
			Consistently(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      utilities.GetTokenxEgressName(securityConfigName, config.Get().TokenxName),
					Namespace: namespaceName,
				}, netpol)
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

		It("should recreate owned resources when they are deleted", func() {
			By("Reconciling the SecurityConfig to create owned resources")

			fakeRecorder := record.NewFakeRecorder(100)
			controllerReconciler := getSecurityConfigReconciler(fakeRecorder)

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the owned NetworkPolicy resource")
			netpol := &v1.NetworkPolicy{}
			netpolKey := types.NamespacedName{
				Name:      utilities.GetTokenxEgressName(securityConfigName, config.Get().TokenxName),
				Namespace: namespaceName,
			}
			Expect(k8sClient.Get(ctx, netpolKey, netpol)).To(Succeed())
			Expect(k8sClient.Delete(ctx, netpol)).To(Succeed())

			// In a real cluster, the controller would automatically reconcile when it detects the Jwker deletion. However, in envtest we need to manually trigger the reconciliation to simulate this behavior.
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that the NetworkPolicy resource is recreated")
			Eventually(func() error {
				return k8sClient.Get(ctx, netpolKey, netpol)
			}).Should(Succeed())

			By("Deleting the owned Jwker resource")
			jwker := &naisiov1.Jwker{}
			jwkerKey := types.NamespacedName{
				Name:      utilities.GetJwkerName(skiperatorAppName),
				Namespace: namespaceName,
			}
			Expect(k8sClient.Get(ctx, jwkerKey, jwker)).To(Succeed())
			Expect(k8sClient.Delete(ctx, jwker)).To(Succeed())

			// In a real cluster, the controller would automatically reconcile when it detects the Jwker deletion. However, in envtest we need to manually trigger the reconciliation to simulate this behavior.
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that the Jwker resource is recreated")
			Eventually(func() error {
				return k8sClient.Get(ctx, jwkerKey, jwker)
			}).Should(Succeed())
		})
	})
})

func getSecurityConfigReconciler(
	eventRecorder record.EventRecorder,
) *SecurityConfigReconciler {
	return &SecurityConfigReconciler{
		Client:   gvkInjectingClient{k8sClient},
		Scheme:   gvkInjectingClient{k8sClient}.Scheme(),
		Recorder: eventRecorder,
	}
}

// We need a wrapper of client.Client in order to override Get() to inject GroupVersionKind of SecurityConfig.
// This is needed as envtest does not populate TypeMeta on Get().
type gvkInjectingClient struct {
	client.Client
}

func (c gvkInjectingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := c.Client.Get(ctx, key, obj, opts...); err != nil {
		return err
	}
	if _, ok := obj.(*accesseratorv1alpha.SecurityConfig); ok {
		obj.GetObjectKind().SetGroupVersionKind(accesseratorv1alpha.GroupVersion.WithKind("SecurityConfig"))
	}
	return nil
}
