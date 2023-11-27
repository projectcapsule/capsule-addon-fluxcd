//go:build e2e

package utils

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	helmaction "helm.sh/helm/v3/pkg/action"
	helmloader "helm.sh/helm/v3/pkg/chart/loader"
	helmcli "helm.sh/helm/v3/pkg/cli"
	registry "helm.sh/helm/v3/pkg/registry"
	helmrelease "helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kind/pkg/cluster"
)

func EnsureKindCluster(name, imageName string, wait time.Duration, kubeConfigPath ...string) (string, error) {
	provider := cluster.NewProvider(
		cluster.ProviderWithDocker(),
	)

	// create the cluster
	if err := provider.Create(
		name,
		cluster.CreateWithNodeImage(imageName),
		cluster.CreateWithWaitForReady(wait),
		cluster.CreateWithDisplayUsage(true),
		cluster.CreateWithDisplaySalutation(true),
	); err != nil {
		return "", errors.Wrap(err, "failed to create cluster")
	}

	kubeconfig, err := provider.KubeConfig(name, false)
	if err != nil {
		return "", err
	}

	if len(kubeConfigPath) > 0 {
		if err = provider.ExportKubeConfig(name, kubeConfigPath[0], false); err != nil {
			return "", err
		}
	}

	return kubeconfig, nil
}

func DeleteKindCluster(name string) error {
	provider := cluster.NewProvider(
		cluster.ProviderWithDocker(),
	)
	if err := provider.Delete(name, ""); err != nil {
		return err
	}

	return nil
}

func HelmInstall(releaseName, releaseNamespace, chart, version string, values map[string]interface{},
	kubeConfig string, timeout time.Duration) (*helmrelease.Release, error) {
	restClientGetter := NewRESTClientGetter("", kubeConfig)

	actionConfig := new(helmaction.Configuration)
	if err := actionConfig.Init(
		restClientGetter,
		releaseNamespace,
		os.Getenv("HELM_DRIVER"),
		func(format string, v ...interface{}) {
			fmt.Sprintf(format, v)
		}); err != nil {
		return nil, err
	}

	registryClient, err := registry.NewClient()
	if err != nil {
		return nil, err
	}
	actionConfig.RegistryClient = registryClient
	restClientGetter.Namespace = releaseNamespace

	client := helmaction.NewInstall(actionConfig)
	client.Namespace = releaseNamespace
	client.ReleaseName = releaseName
	client.ChartPathOptions.Version = version
	client.CreateNamespace = true
	client.Wait = true
	client.WaitForJobs = true
	client.Timeout = timeout

	cp, err := client.ChartPathOptions.LocateChart(chart, helmcli.New())
	if err != nil {
		return nil, err
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := helmloader.Load(cp)
	if err != nil {
		return nil, err
	}

	rel, err := client.Run(chartRequested, values)
	if err != nil {
		return nil, err
	}

	return rel, nil
}

type SimpleRESTClientGetter struct {
	Namespace  string
	KubeConfig string
}

func NewRESTClientGetter(namespace, kubeConfig string) *SimpleRESTClientGetter {
	return &SimpleRESTClientGetter{
		Namespace:  namespace,
		KubeConfig: kubeConfig,
	}
}

func (c *SimpleRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(c.KubeConfig))
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (c *SimpleRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := c.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	// The more groups you have, the more discovery requests you need to make.
	// given 25 groups (our groups + a few custom conf) with one-ish version each, discovery needs to make 50 requests
	// double it just so we don't end up here again for a while.  This config is only used for discovery.
	config.Burst = 100

	discoveryClient, _ := discovery.NewDiscoveryClientForConfig(config)
	return memory.NewMemCacheClient(discoveryClient), nil
}

func (c *SimpleRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := c.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

func (c *SimpleRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// Use the standard defaults for this client command.
	// DEPRECATED: remove and replace with something more accurate.
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}
	overrides.Context.Namespace = c.Namespace

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}

type E2eClient struct {
	client.Client
}

func (e *E2eClient) sleep() {
	time.Sleep(500 * time.Millisecond)
}

func (e *E2eClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	defer e.sleep()

	return e.Client.Get(ctx, key, obj, opts...)
}

func (e *E2eClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	defer e.sleep()

	return e.Client.List(ctx, list, opts...)
}

func (e *E2eClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	defer e.sleep()

	return e.Client.Create(ctx, obj, opts...)
}

func (e *E2eClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	defer e.sleep()

	return e.Client.Delete(ctx, obj, opts...)
}

func (e *E2eClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	defer e.sleep()

	return e.Client.Update(ctx, obj, opts...)
}

func (e *E2eClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	defer e.sleep()

	return e.Client.Patch(ctx, obj, patch, opts...)
}

func (e *E2eClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	defer e.sleep()

	return e.Client.DeleteAllOf(ctx, obj, opts...)
}
