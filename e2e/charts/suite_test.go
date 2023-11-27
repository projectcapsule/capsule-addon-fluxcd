//go:build e2e

package charts

import (
	"fmt"
	"github.com/maxgio92/capsule-addon-flux/e2e/utils"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	capsulev1beta2 "github.com/projectcapsule/capsule/api/v1beta2"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	k8sVersionDefault = "v1.27.3"

	KindClusterName   = "capsule-addon-fluxcd-e2e"
	KindNodeImageRepo = "kindest/node"

	HelmChartVersionCapsule      = "0.6.0"
	HelmChartVersionCapsuleProxy = "0.5.2"
	HelmChartURLCapsule          = "oci://ghcr.io/projectcapsule/charts/capsule"
	HelmChartURLCapsuleProxy     = "oci://ghcr.io/projectcapsule/charts/capsule-proxy"
	NamespaceCapsule             = "capsule-system"
	NamespaceCapsuleProxy        = "capsule-system"

	EnvKubernetesVersion        = "KUBERNETES_VERSION"
	EnvChartVersionCapsule      = "CAPSULE_HELM_VERSION"
	EnvChartVersionCapsuleProxy = "CAPSULE_PROXY_HELM_VERSION"

	TimeoutKindSeconds        = 120
	TimeoutHelmInstallSeconds = 240

	KubeConfigPath = "/tmp/kubeconfig"

	TenantName            = "oil"
	TenantSystemNamespace = "oil-system"
	TenantOwnerSAName     = "gitops-reconciler"
)

var (
	adminConfig *rest.Config
	adminClient client.Client
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	k8sVersion := os.Getenv(EnvKubernetesVersion)
	if k8sVersion == "" {
		k8sVersion = k8sVersionDefault
	}

	capsuleHelmVersion := os.Getenv(EnvChartVersionCapsule)
	if capsuleHelmVersion == "" {
		capsuleHelmVersion = HelmChartVersionCapsule
	}

	capsuleProxyHelmVersion := os.Getenv(EnvChartVersionCapsuleProxy)
	if capsuleProxyHelmVersion == "" {
		capsuleProxyHelmVersion = HelmChartVersionCapsuleProxy
	}

	var (
		err        error
		kubeConfig string
	)

	By("Ensuring a Kind cluster", func() {
		kubeConfig, err = utils.EnsureKindCluster(
			KindClusterName,
			fmt.Sprintf("%s:%s", KindNodeImageRepo, k8sVersion),
			time.Second*TimeoutKindSeconds,
			KubeConfigPath,
		)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(kubeConfig).ToNot(BeEmpty())

		err = os.Setenv("KUBECONFIG", KubeConfigPath)
		Expect(err).ShouldNot(HaveOccurred())
	})

	By("Ensuring Capsule is installed", func() {
		err = ensureCapsule(capsuleHelmVersion, kubeConfig)
		Expect(err).ShouldNot(HaveOccurred())
	})

	By("Ensuring Capsule Proxy is installed", func() {
		err = ensureCapsuleProxy(capsuleProxyHelmVersion, kubeConfig)
		Expect(err).ShouldNot(HaveOccurred())
	})

	By("Building the e2e client", func() {
		adminConfig, err = config.GetConfig()
		Expect(err).ToNot(HaveOccurred())
		Expect(adminConfig).ToNot(BeNil())

		Expect(scheme.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
		Expect(capsulev1beta2.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

		c, err := client.New(adminConfig, client.Options{Scheme: scheme.Scheme})
		Expect(err).ToNot(HaveOccurred())
		Expect(c).ToNot(BeNil())

		adminClient = &utils.E2eClient{Client: c}
	})

	By("Ensuring the addon is installed", func() {
		_, err := utils.HelmInstall(
			"capsule-addon-fluxcd",
			NamespaceCapsule,
			"../charts/capsule-addon-fluxcd",
			"",
			nil,
			kubeConfig,
			TimeoutHelmInstallSeconds*time.Second)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = AfterSuite(func() {
	Expect(utils.DeleteKindCluster(KindClusterName)).ShouldNot(HaveOccurred())
})

func ensureCapsule(version, kubeConfig string) error {
	if _, err := utils.HelmInstall(
		"capsule",
		NamespaceCapsule,
		HelmChartURLCapsule,
		version,
		nil,
		kubeConfig,
		TimeoutHelmInstallSeconds*time.Second); err != nil {
		return errors.Wrap(err, "error installing capsule-proxy")
	}
	return nil
}

func ensureCapsuleProxy(version, kubeConfig string) error {
	if _, err := utils.HelmInstall(
		"capsule-proxy",
		NamespaceCapsuleProxy,
		HelmChartURLCapsuleProxy,
		version,
		nil,
		kubeConfig,
		TimeoutHelmInstallSeconds*time.Second); err != nil {
		return errors.Wrap(err, "error installing capsule-proxy")
	}
	return nil
}
