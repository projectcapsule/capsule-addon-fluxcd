package manager

import (
	"flag"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	capsulev1beta2 "github.com/projectcapsule/capsule/api/v1beta2"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/projectcapsule/capsule-addon-flux/pkg/controller/serviceaccount"
	"github.com/projectcapsule/capsule-addon-flux/pkg/indexer"
)

type Options struct {
	ProxyURL    string
	ProxyCAPath string

	SetupLog logr.Logger
	Zo       *zap.Options
}

func New() *cobra.Command {
	setupLog := ctrl.Log.WithName("setup")

	zo := &zap.Options{
		EncoderConfigOptions: append([]zap.EncoderConfigOption{}, func(config *zapcore.EncoderConfig) {
			config.EncodeTime = zapcore.ISO8601TimeEncoder
		}),
	}

	opts := &Options{
		SetupLog: setupLog,
		Zo:       zo,
	}

	cmd := &cobra.Command{
		Use:   "manager",
		Short: "Starts the manager of the Capsule addon for FluxCD",
		RunE:  opts.Run,
	}

	// Add Proxy options.
	cmd.Flags().StringVar(&opts.ProxyURL, "proxy-url", "https://capsule-proxy.capsule-system.svc:9001", "Kubernetes Service URL on which Capsule Proxy is waiting for connections")
	cmd.Flags().StringVar(&opts.ProxyCAPath, "proxy-ca-path", "/tmp/ca.crt", "File containing the Certificate Authority used by Capsule Proxy")

	// Add Zap options.
	var fs flag.FlagSet
	opts.Zo.BindFlags(&fs)
	cmd.Flags().AddGoFlagSet(&fs)

	return cmd
}

func (o *Options) Run(_ *cobra.Command, _ []string) error {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return errors.Wrap(err, "unable to add client-go types to the manager's scheme")
	}
	if err := capsulev1beta2.AddToScheme(scheme); err != nil {
		return errors.Wrap(err, "unable to add Capsule types to the manager's scheme")
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(o.Zo)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: fmt.Sprintf(":%d", PortManagerMetricsServer),
		},
		HealthProbeBindAddress: fmt.Sprintf(":%d", PortManagerHealthProbe),
		NewClient: func(config *rest.Config, options client.Options) (client.Client, error) {
			options.Cache.Unstructured = true

			return client.New(config, options)
		},
	})
	if err != nil {
		o.SetupLog.Error(err, "unable to create manager")
		return errors.Wrap(err, "unable to create manager")
	}

	_ = mgr.AddReadyzCheck("ping", healthz.Ping)
	_ = mgr.AddHealthzCheck("ping", healthz.Ping)

	proxyCA, err := os.ReadFile(o.ProxyCAPath)
	if err != nil {
		return errors.Wrap(err, "unable to read the CA file")
	}

	ctx := ctrl.SetupSignalHandler()

	if err = indexer.AddToManager(ctx, o.SetupLog, mgr); err != nil {
		o.SetupLog.Error(err, "unable to setup indexers")
		return errors.Wrap(err, "unable to setup indexers")
	}

	if err = serviceaccount.NewServiceAccountReconciler(
		serviceaccount.WithClient(mgr.GetClient()),
		serviceaccount.WithLogger(ctrl.Log.WithName("controller").WithName("ServiceAccount")),
		serviceaccount.WithProxyCA(string(proxyCA)),
		serviceaccount.WithProxyURL(o.ProxyURL),
	).SetupWithManager(ctx, mgr); err != nil {
		o.SetupLog.Error(err, "unable to create manager", "controller", "ServiceAccount")
		return errors.Wrap(err, "unable to setup the service account controller")
	}

	if err = mgr.Start(ctx); err != nil {
		o.SetupLog.Error(err, "problem running manager")
		return errors.Wrap(err, "unable to start the manager")
	}

	return nil
}
