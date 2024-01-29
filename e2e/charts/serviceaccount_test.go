//go:build e2e

package charts

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	capsulev1beta2 "github.com/projectcapsule/capsule/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/tools/clientcmd"
	"time"

	"github.com/projectcapsule/capsule-addon-flux/pkg/controller/serviceaccount"
)

var _ = Describe("Creating a new ServiceAccount", func() {
	var (
		sa                        *corev1.ServiceAccount
		configSecret, tokenSecret *corev1.Secret
		err                       error
	)

	Context("in the Tenant system Namespace", Ordered, func() {
		BeforeAll(func() {
			Expect(adminClient.Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: TenantSystemNamespace,
				},
			})).Should(Succeed())
		})

		AfterAll(func() {
			Eventually(func() error {
				return adminClient.Delete(context.TODO(), &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: TenantSystemNamespace,
					},
				})
			}).Should(Succeed())
		})

		Context("set as owner of the Tenant", func() {
			BeforeEach(func() {
				Expect(adminClient.Create(context.TODO(), &capsulev1beta2.Tenant{
					ObjectMeta: metav1.ObjectMeta{
						Name: TenantName,
					},
					Spec: capsulev1beta2.TenantSpec{
						Owners: []capsulev1beta2.OwnerSpec{
							{
								Kind: "ServiceAccount",
								Name: fmt.Sprintf("system:serviceaccount:%s:%s",
									TenantSystemNamespace, TenantOwnerSAName),
							},
						},
					},
				})).Should(Succeed())
			})
			AfterEach(func() {
				Expect(adminClient.Delete(context.TODO(), &capsulev1beta2.Tenant{
					ObjectMeta: metav1.ObjectMeta{
						Name: TenantName,
					},
				})).Should(Succeed())
			})

			When("has the annotation to enable the addon", func() {
				BeforeEach(func() {
					sa = &corev1.ServiceAccount{
						ObjectMeta: metav1.ObjectMeta{
							Name:      TenantOwnerSAName,
							Namespace: TenantSystemNamespace,
							Annotations: map[string]string{
								serviceaccount.ServiceAccountAddonAnnotationKey: serviceaccount.ServiceAccountAddonAnnotationValue,
							},
						},
					}
					err = adminClient.Create(context.TODO(), sa)
					Expect(err).ShouldNot(HaveOccurred())
				})

				AfterEach(func() {
					Expect(adminClient.Delete(context.TODO(), sa)).Should(Succeed())
				})

				It("should generate the Tenant system Namespace RoleBinding", func() {
					Eventually(func(g Gomega) {
						rb := new(rbacv1.RoleBinding)
						err = adminClient.Get(context.TODO(), types.NamespacedName{
							Namespace: TenantSystemNamespace,
							Name:      TenantOwnerSAName,
						}, rb)
						g.Expect(err).Should(Succeed())
						g.Expect(rb.RoleRef.Kind).To(Equal("ClusterRole"))
						g.Expect(rb.RoleRef.Name).To(Equal("cluster-admin"))

						By("binding it to the Tenant Owner", func() {
							g.Expect(len(rb.Subjects)).To(Equal(1))
							g.Expect(rb.Subjects[0].Kind).To(Equal("ServiceAccount"))
							g.Expect(rb.Subjects[0].Name).To(Equal(TenantOwnerSAName))
							g.Expect(rb.Subjects[0].Namespace).To(Equal(TenantSystemNamespace))
						})
					}, 20*time.Second, 1*time.Second).Should(Succeed())
				})

				It("should generate the impersonator ClusterRole", func() {
					Eventually(func(g Gomega) {
						cr := new(rbacv1.ClusterRole)
						err = adminClient.Get(context.TODO(), types.NamespacedName{
							Name: fmt.Sprintf("%s-%s-impersonator",
								TenantSystemNamespace, TenantOwnerSAName),
						}, cr)
						g.Expect(err).Should(Succeed())
						g.Expect(len(cr.Rules)).To(Equal(1))
					}, 20*time.Second, 1*time.Second).Should(Succeed())
				})

				It("should generate the impersonator ClusterRoleBinding", func() {
					Eventually(func(g Gomega) {
						crb := new(rbacv1.ClusterRoleBinding)
						err = adminClient.Get(context.TODO(), types.NamespacedName{
							Name: fmt.Sprintf("%s-%s-impersonator",
								TenantSystemNamespace, TenantOwnerSAName),
						}, crb)
						g.Expect(err).Should(Succeed())
						g.Expect(crb.RoleRef.Kind).To(Equal("ClusterRole"))
						g.Expect(crb.RoleRef.Name).To(Equal(
							fmt.Sprintf("%s-%s-impersonator", TenantSystemNamespace, TenantOwnerSAName),
						))

						By("binding it to the Tenant Owner", func() {
							g.Expect(len(crb.Subjects)).To(Equal(1))
							g.Expect(crb.Subjects[0].Kind).To(Equal("ServiceAccount"))
							g.Expect(crb.Subjects[0].Name).To(Equal(TenantOwnerSAName))
							g.Expect(crb.Subjects[0].Namespace).To(Equal(TenantSystemNamespace))
						})
					}, 20*time.Second, 1*time.Second).Should(Succeed())
				})

				It("should generate the kubeConfig Secret", func() {
					Eventually(func(g Gomega) {
						configSecret = new(corev1.Secret)
						err = adminClient.Get(context.TODO(), types.NamespacedName{
							Namespace: TenantSystemNamespace,
							Name: fmt.Sprintf("%s%s",
								TenantOwnerSAName, serviceaccount.SecretNameSuffixKubeconfig),
						}, configSecret)
						g.Expect(err).Should(Succeed())
						g.Expect(configSecret.Data).ToNot(BeNil())
						g.Expect(configSecret.Data[serviceaccount.SecretKeyKubeconfig]).ToNot(BeEmpty())
					}, 20*time.Second, 1*time.Second).Should(Succeed())
				})

				It("should generate the Service Account token Secret", func() {
					Eventually(func(g Gomega) {
						tokenSecret = new(corev1.Secret)
						err = adminClient.Get(context.TODO(), types.NamespacedName{
							Namespace: TenantSystemNamespace,
							Name: fmt.Sprintf("%s%s",
								TenantOwnerSAName, serviceaccount.SecretNameSuffixToken),
						}, tokenSecret)
						g.Expect(err).Should(Succeed())
						g.Expect(tokenSecret.Data).ToNot(BeNil())
						g.Expect(tokenSecret.Data[corev1.ServiceAccountTokenKey]).ToNot(BeEmpty())
					}, 20*time.Second, 1*time.Second).Should(Succeed())
				})
			})

			When("has the annotation to enable the addon and the annotation to make the kubeconfig global", func() {
				BeforeEach(func() {
					sa = &corev1.ServiceAccount{
						ObjectMeta: metav1.ObjectMeta{
							Name:      TenantOwnerSAName,
							Namespace: TenantSystemNamespace,
							Annotations: map[string]string{
								serviceaccount.ServiceAccountAddonAnnotationKey:  serviceaccount.ServiceAccountAddonAnnotationValue,
								serviceaccount.ServiceAccountGlobalAnnotationKey: serviceaccount.ServiceAccountGlobalAnnotationValue,
							},
						},
					}
					err = adminClient.Create(context.TODO(), sa)
					Expect(err).ShouldNot(HaveOccurred())
				})

				AfterEach(func() {
					Expect(adminClient.Delete(context.TODO(), sa)).Should(Succeed())
				})

				It("should generate GlobalTenantResource with the kubeConfig Secret", func() {
					Eventually(func(g Gomega) {
						gtr := new(capsulev1beta2.GlobalTenantResource)
						err = adminClient.Get(context.TODO(), types.NamespacedName{
							Name: fmt.Sprintf("%s-%s%s",
								TenantName, TenantOwnerSAName, serviceaccount.GlobalTenantResourceSuffix),
						}, gtr)
						g.Expect(err).Should(Succeed())
						g.Expect(len(gtr.Spec.Resources)).To(Equal(1))
						g.Expect(gtr.Spec.TenantSelector.MatchLabels["kubernetes.io/metadata.name"]).To(Equal(
							TenantName,
						))

						By("setting the server to Capsule Proxy", func() {
							kcSecret := &corev1.Secret{}
							g.Expect(len(gtr.Spec.Resources[0].RawItems)).To(Equal(1))
							g.Expect(json.Unmarshal(gtr.Spec.Resources[0].RawItems[0].Raw, kcSecret)).To(Succeed())
							g.Expect(kcSecret.Data[serviceaccount.SecretKeyKubeconfig]).ToNot(BeEmpty())

							kc, err := clientcmd.Load(kcSecret.Data[serviceaccount.SecretKeyKubeconfig])
							g.Expect(err).Should(Succeed())
							g.Expect(kc).ToNot(BeNil())
							g.Expect(kc.Clusters[serviceaccount.KubeconfigClusterName].Server).To(Equal(
								fmt.Sprintf("https://capsule-proxy.%s.svc:9001", NamespaceCapsuleProxy),
							))
							g.Expect(kc.AuthInfos[serviceaccount.KubeconfigUserName].Token).ToNot(BeEmpty())
						})
					}, 20*time.Second, 1*time.Second).Should(Succeed())
				})
			})
		})
	})
})
