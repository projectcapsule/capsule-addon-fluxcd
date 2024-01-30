//go:build e2e

// Copyright 2020-2024 Project Capsule Authors.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	capsulev1beta2 "github.com/projectcapsule/capsule/api/v1beta2"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	cmd "github.com/projectcapsule/capsule-addon-flux/cmd/manager"
	"github.com/projectcapsule/capsule-addon-flux/e2e/utils"
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
	TimeoutHelmInstallSeconds = 120

	KubeConfigPath = "/tmp/kubeconfig"

	TenantName            = "oil"
	TenantSystemNamespace = "oil-system"
	TenantOwnerSAName     = "gitops-reconciler"

	CapsuleProxyCAFilePath = "/tmp/capsule-proxy-ca.crt"
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
		err = EnsureCapsule(capsuleHelmVersion, kubeConfig)
		Expect(err).ShouldNot(HaveOccurred())
	})

	By("Ensuring Capsule Proxy is installed", func() {
		err = EnsureCapsuleProxy(capsuleProxyHelmVersion, kubeConfig)
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

	By("Retrieving the CA of the Capsule Proxy", func() {
		caSecret := new(corev1.Secret)
		err = adminClient.Get(context.TODO(), types.NamespacedName{
			Namespace: "capsule-system",
			Name:      "capsule-proxy",
		}, caSecret)
		Expect(err).ToNot(HaveOccurred())
		Expect(caSecret).ToNot(BeNil())
		Expect(caSecret.Data).ToNot(BeNil())
		Expect(caSecret.Data["ca"]).ToNot(BeEmpty())

		err = os.WriteFile(CapsuleProxyCAFilePath, caSecret.Data["ca"], 0644)
		Expect(err).ToNot(HaveOccurred())
	})

	By("Starting the manager", func() {
		mo := &cmd.Options{
			ProxyURL:    fmt.Sprintf("https://capsule-proxy.%s.svc:9001", NamespaceCapsuleProxy),
			ProxyCAPath: CapsuleProxyCAFilePath,
			SetupLog:    ctrl.Log.WithName("setup"),
			Zo: &zap.Options{
				EncoderConfigOptions: append([]zap.EncoderConfigOption{}, func(config *zapcore.EncoderConfig) {
					config.EncodeTime = zapcore.ISO8601TimeEncoder
				}),
			},
		}
		go func() {
			err = mo.Run(nil, nil)
			Expect(err).ShouldNot(HaveOccurred())
		}()
	})
})

var _ = AfterSuite(func() {
	Expect(utils.DeleteKindCluster(KindClusterName)).ShouldNot(HaveOccurred())
	Expect(os.Remove(CapsuleProxyCAFilePath)).ShouldNot(HaveOccurred())
})

func EnsureCapsule(version, kubeConfig string) error {
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

func EnsureCapsuleProxy(version, kubeConfig string) error {
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
